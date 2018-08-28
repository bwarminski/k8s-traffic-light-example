/*
Copyright 2018 Brett Warminski.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stoplight

import (
	"context"
	"log"
	"reflect"

	"fmt"
	safetyv1alpha1 "github.com/bwarminski/k8s-traffic-light-example/pkg/apis/safety/v1alpha1"
	"github.com/bwarminski/k8s-traffic-light-example/pkg/controller/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"
)

// Add creates a new Stoplight Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this safety.Add(mgr) to install this Controller
func Add(mgr manager.Manager, d util.ControllerDependencies) error {
	return add(mgr, newReconciler(mgr, d))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, d util.ControllerDependencies) *ReconcileStoplight {
	return &ReconcileStoplight{Client: mgr.GetClient(), scheme: mgr.GetScheme(), events: make(chan event.GenericEvent), clock: d.Clock}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileStoplight) error {
	// Create a new controller
	c, err := controller.New("stoplight-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Stoplight
	err = c.Watch(&source.Kind{Type: &safetyv1alpha1.Stoplight{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	mgr.GetCache().IndexField(&safetyv1alpha1.Ambulance{}, "composite.crossingWithLightsOn", client.IndexerFunc(func(o runtime.Object) []string {
		var result []string
		ambulance, ok := o.(*safetyv1alpha1.Ambulance)
		if ok && ambulance.Spec.LightsOn {
			result = append(result, ambulance.Spec.Crossing)
		}
		return result
	}))

	ambulancesToLights := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			ambulance, ok := a.Object.(*safetyv1alpha1.Ambulance)
			var results []reconcile.Request
			if !ok || ambulance.Spec.Crossing == "" {
				return results
			}

			// Enqueue the light that the ambulance is intending to cross
			results = append(results, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ambulance.Spec.Crossing,
					Namespace: ambulance.Namespace,
				},
			})

			// Optimization - Enqueue any other lights that were waiting on this ambulance
			listOpts := &client.ListOptions{Namespace: ambulance.Namespace}
			err := listOpts.SetLabelSelector(fmt.Sprintf("ambulance.%s", ambulance.Name))
			if err != nil {
				log.Printf("Error setting label selector - %s", err.Error())
				return results
			}
			stoplights := &safetyv1alpha1.StoplightList{}
			err = r.Client.List(context.TODO(), listOpts, stoplights)
			if err == nil {
				for _, stoplight := range stoplights.Items {
					results = append(results, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      stoplight.Name,
							Namespace: stoplight.Namespace,
						},
					})
				}
			} else {
				log.Printf("Error listing stoplights - %s", err.Error())
			}
			return results
		})

	err = c.Watch(
		&source.Kind{Type: &safetyv1alpha1.Ambulance{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: ambulancesToLights,
		})

	if err != nil {
		return err
	}

	// Watch for inbound events from the controller's events channel. This will be used to enqueue a stoplight
	// when it's time for it to make a transition.
	// This is wrapped as a Runnable because registration of this watch can't occur until the manager has been initialized
	// with a stop channel
	mgr.Add(manager.RunnableFunc(func(stop <-chan struct{}) error {
		innerErr := c.Watch(
			&source.Channel{Source: r.events},
			&handler.EnqueueRequestForObject{},
		)

		if innerErr != nil {
			return innerErr
		}

		// Block until channel closes
		select {
		case <-stop:
			return nil
		}

	}))
	return nil
}

var _ reconcile.Reconciler = &ReconcileStoplight{}

// ReconcileStoplight reconciles a Stoplight object
type ReconcileStoplight struct {
	client.Client
	scheme *runtime.Scheme
	events chan event.GenericEvent
	clock  util.Clock
}

// Reconcile reads that state of the cluster for a Stoplight object and makes changes based on the state read
// and what is in the Stoplight.Spec

// +kubebuilder:rbac:groups=safety.traffic.bwarminski.io,resources=stoplights,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=safety.traffic.bwarminski.io,resources=ambulances,verbs=get;list;watch
func (r *ReconcileStoplight) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Stoplight instance
	stoplight := &safetyv1alpha1.Stoplight{}
	err := r.Get(context.TODO(), request.NamespacedName, stoplight)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	ambulances := &safetyv1alpha1.AmbulanceList{}
	listOpts := &client.ListOptions{Namespace: stoplight.Namespace}
	//err = listOpts.SetFieldSelector(fmt.Sprintf("spec.crossing=%s,spec.lightsOn=true", stoplight.Name))
	err = listOpts.SetFieldSelector(fmt.Sprintf("composite.crossingWithLightsOn=%s", stoplight.Name))
	if err != nil {
		log.Printf("Error setting ambulance field selector - %s", err.Error())
		return reconcile.Result{}, err
	}
	if err = r.List(context.TODO(), listOpts, ambulances); err != nil {
		log.Printf("Error listing ambulances - %s", err.Error())
		return reconcile.Result{}, err
	}

	now := r.clock.Now()

	desiredStatus := stoplight.Status.DeepCopy()

	if len(ambulances.Items) > 0 {
		desiredStatus.EmergencyMode = true
	}

	// TODO: Always enqueue reminders
	switch stoplight.Status.Color {
	case safetyv1alpha1.Red:
		if !desiredStatus.EmergencyMode && stoplight.Status.LastTransition+stoplight.Spec.RedLightDurationSec <= now {
			desiredStatus.Color = safetyv1alpha1.Green
			desiredStatus.LastTransition = now
			r.enqueueReminder(now+stoplight.Spec.GreenLightDurationSec, types.NamespacedName{Namespace: stoplight.Namespace, Name: stoplight.Name})
		}
	case safetyv1alpha1.Yellow:
		if stoplight.Status.LastTransition+stoplight.Spec.YellowLightDurationSec <= now {
			desiredStatus.Color = safetyv1alpha1.Red
			desiredStatus.LastTransition = now
			r.enqueueReminder(now+stoplight.Spec.RedLightDurationSec, types.NamespacedName{Namespace: stoplight.Namespace, Name: stoplight.Name})
		}
	case safetyv1alpha1.Green:
		if desiredStatus.EmergencyMode || stoplight.Status.LastTransition+stoplight.Spec.GreenLightDurationSec <= now {
			desiredStatus.Color = safetyv1alpha1.Yellow
			desiredStatus.LastTransition = now
			r.enqueueReminder(now+stoplight.Spec.YellowLightDurationSec, types.NamespacedName{Namespace: stoplight.Namespace, Name: stoplight.Name})
		}
	default:
		desiredStatus.LastTransition = now
		desiredStatus.Color = safetyv1alpha1.Red
		r.enqueueReminder(now+stoplight.Spec.RedLightDurationSec, types.NamespacedName{Namespace: stoplight.Namespace, Name: stoplight.Name})
	}

	desiredLabels := make(map[string]string)
	if stoplight.Labels != nil {
		for _, a := range ambulances.Items {
			// Indicate that we're watching for this ambulance
			desiredLabels[fmt.Sprintf("ambulance.%s", a.Name)] = "approaching"
		}
		for k, v := range stoplight.Labels {
			if !strings.HasPrefix(k, "ambulance.") {
				desiredLabels[k] = v
			}
		}
	}

	// Update our status and labels if they've changed
	labelsPopulated := len(stoplight.Labels) > 0 || len(desiredLabels) > 0
	if !reflect.DeepEqual(stoplight.Status, *desiredStatus) || (labelsPopulated && !reflect.DeepEqual(stoplight.Labels, desiredLabels)) {
		stoplight.Status = *desiredStatus
		stoplight.Labels = desiredLabels

		log.Printf("Updating Stoplight %s/%s to %s\n", stoplight.Namespace, stoplight.Name, stoplight.Status.Color)
		err = r.Update(context.TODO(), stoplight)
		if err != nil {
			log.Printf("Error updating Stoplight - %s", err.Error())
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// Defers a function that will requeue our stoplight at a certain point in the future
func (r *ReconcileStoplight) enqueueReminder(when int64, name types.NamespacedName) {
	now := r.clock.Now()
	go func() {
		if when > now {
			time.Sleep(time.Second * time.Duration(when-now))
		}
		stoplight := &safetyv1alpha1.Stoplight{}
		// This is a little silly - we'll end up discarding and fetching it again, but whatever, makes for an easy example
		err := r.Get(context.TODO(), name, stoplight)
		if err != nil {
			log.Printf("Unable to process timer for %s/%s - %s\n", name.Namespace, name.Name, err.Error())
			return
		}
		r.events <- event.GenericEvent{Meta: stoplight.GetObjectMeta(), Object: stoplight}
	}()
}
