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

package crosswalksignal

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
	"time"
)

// Add creates a new CrosswalkSignal Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, d util.ControllerDependencies) error {
	return add(mgr, newReconciler(mgr, d))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, d util.ControllerDependencies) *ReconcileCrosswalkSignal {
	return &ReconcileCrosswalkSignal{Client: mgr.GetClient(), scheme: mgr.GetScheme(), events: make(chan event.GenericEvent), clock: d.Clock}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r *ReconcileCrosswalkSignal) error {
	// Create a new controller
	c, err := controller.New("crosswalksignal-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CrosswalkSignal
	err = c.Watch(&source.Kind{Type: &safetyv1alpha1.CrosswalkSignal{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	stoplightsToSignals := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			stoplight, ok := a.Object.(*safetyv1alpha1.Stoplight)
			var results []reconcile.Request
			if !ok {
				return results
			}

			// Find all cross walks protecting this stoplight
			listOpts := &client.ListOptions{Namespace: stoplight.Namespace}
			err := listOpts.SetFieldSelector(fmt.Sprintf("spec.stoplight = %s", stoplight.Name))
			if err != nil {
				log.Printf("Error setting field selector - %s", err.Error())
				return results
			}
			signals := &safetyv1alpha1.CrosswalkSignalList{}
			err = r.Client.List(context.TODO(), listOpts, signals)
			if err == nil {
				for _, signal := range signals.Items {
					results = append(results, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      signal.Name,
							Namespace: signal.Namespace,
						},
					})
				}
			} else {
				log.Printf("Error listing signals - %s", err.Error())
			}
			return results
		})

	err = c.Watch(&source.Kind{Type: &safetyv1alpha1.Stoplight{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: stoplightsToSignals,
	})
	if err != nil {
		return err
	}

	// Watch for inbound events from the controller's events channel. This will be used to enqueue a crosswalk signal
	// when it's time for it to make a transition.
	return c.Watch(
		&source.Channel{Source: r.events},
		&handler.EnqueueRequestForObject{},
	)
}

var _ reconcile.Reconciler = &ReconcileCrosswalkSignal{}

// ReconcileCrosswalkSignal reconciles a CrosswalkSignal object
type ReconcileCrosswalkSignal struct {
	client.Client
	scheme *runtime.Scheme
	events chan event.GenericEvent
	clock  util.Clock
}

// Reconcile reads that state of the cluster for a CrosswalkSignal object and makes changes based on the state read
// and what is in the CrosswalkSignal.Spec

// +kubebuilder:rbac:groups=safety.traffic.bwarminski.io,resources=crosswalksignals,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=safety.traffic.bwarminski.io,resources=stoplights,verbs=get;list;watch
func (r *ReconcileCrosswalkSignal) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CrosswalkSignal instance
	signal := &safetyv1alpha1.CrosswalkSignal{}
	err := r.Get(context.TODO(), request.NamespacedName, signal)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Search for the stoplight we care about
	var stoplight *safetyv1alpha1.Stoplight
	var nextGreen int64
	var nextTransition int64

	if signal.Spec.Stoplight != "" {
		stoplight = &safetyv1alpha1.Stoplight{}
		err = r.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: signal.Spec.Stoplight}, stoplight)
		if err != nil {
			if !errors.IsNotFound(err) {
				return reconcile.Result{}, err
			} else {
				// Not found, clear it out. Option[Stoplight] would be nifty
				stoplight = nil
			}
		} else {
			nextGreen = stoplight.Status.LastTransition + stoplight.Spec.RedLightDurationSec
			nextTransition = nextGreen - signal.Spec.FlashingHandDurationSec - signal.Spec.GreenLightBufferDurationSec
		}
	}

	now := r.clock.Now()
	desiredStatus := signal.Status.DeepCopy()

	switch signal.Status.Symbol {
	case safetyv1alpha1.DontWalk:
		if stoplight != nil && stoplight.Status.Color == safetyv1alpha1.Red && !stoplight.Status.EmergencyMode {
			desiredStatus.Symbol = safetyv1alpha1.Walk
			desiredStatus.LastTransition = now
			r.enqueueReminder(nextTransition, types.NamespacedName{Namespace: signal.Namespace, Name: signal.Name})
		}
	case safetyv1alpha1.Walk:
		if stoplight == nil || stoplight.Status.Color != safetyv1alpha1.Red {
			desiredStatus.Symbol = safetyv1alpha1.DontWalk
			desiredStatus.LastTransition = now
		} else if nextTransition <= now {
			desiredStatus.Symbol = safetyv1alpha1.FlashyHand
			desiredStatus.LastTransition = now
			r.enqueueReminder(now+signal.Spec.FlashingHandDurationSec, types.NamespacedName{Namespace: signal.Namespace, Name: signal.Name})
		}
	case safetyv1alpha1.FlashyHand:
		if stoplight == nil || stoplight.Status.Color != safetyv1alpha1.Red || nextGreen-signal.Spec.GreenLightBufferDurationSec <= now {
			desiredStatus.Symbol = safetyv1alpha1.DontWalk
			desiredStatus.LastTransition = now
		}
	default:
		desiredStatus.Symbol = safetyv1alpha1.DontWalk
		desiredStatus.LastTransition = now
	}

	// Update the found object and write the result back if there are any changes
	if !reflect.DeepEqual(&signal.Status, desiredStatus) {
		signal.Status = *desiredStatus
		log.Printf("Updating Signal %s/%s to %s\n", signal.Namespace, signal.Name, signal.Status.Symbol)
		err = r.Update(context.TODO(), signal)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// Defers a function that will requeue our signal at a certain point in the future
func (r *ReconcileCrosswalkSignal) enqueueReminder(when int64, name types.NamespacedName) {
	now := r.clock.Now()
	go func() {
		if when > now {
			time.Sleep(time.Second * time.Duration(when-now))
		}
		signal := &safetyv1alpha1.CrosswalkSignal{}
		// This is a little silly - we'll end up discarding and fetching it again, but whatever, makes for an easy example
		// We also copy pasted this exact same code from the stoplight controller
		err := r.Get(context.TODO(), name, signal)
		if err != nil {
			log.Printf("Unable to process timer for %s/%s - %s\n", name.Namespace, name.Name, err.Error())
			return
		}
		r.events <- event.GenericEvent{Meta: signal.GetObjectMeta(), Object: signal}
	}()
}
