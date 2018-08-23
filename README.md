Traffic Lights
--------------

An toy Kubernetes custom resource and controller that illustrates a way to manage state transitions and relationships
between resources.

## Resources

This package defines three resources:

### Stoplights

```
apiVersion: safety.traffic.bwarminski.io/v1alpha1
kind: Stoplight
metadata:
  name: elm-street-north-south
spec:
  greenLightDurationSec: 30
  redLightDurationSec: 30
  yellowLightDurationSec: 10
```

A stoplight resource defines a unique stoplight positioned across an intersection. Stoplights have a set duration
for how long they should remain a color before transitioning to the next, defined by the *DurationSec specifications.

Once operational, a stoplight defines its status in terms of its color and the last time it made a transition in seconds
since epoch:

```
status:
  color: Green
  lastTransition: 1234
  emergencyMode: true
```

The colors of a stoplight are either `Red`, `Yellow`, or `Green`. Any other value indicates that the stoplight is
not operational. For most purposes, this means something like `Flashing Red` but we all know that nobody remembers what
to do when the lights are acting funny anyway, so who cares?

A true value for `emergencyMode` indicates that the light is in the process of turning red and staying red because an
emergency vehicle is travelling through.

### Crosswalk Signals

```
apiVersion: safety.traffic.bwarminski.io/v1alpha1
kind: CrosswalkSignal
metadata:
  name: elm-street-east-west
spec:
  stoplight: elm-street-north-south
  flashingHandDurationSec: 10
  greenLightBufferDurationSec: 10

```

A crosswalk signal defines a unique crosswalk signal placed perpendicular to a stoplight in most cases. Its behavior
is defined by the name of the stoplight it is protecting against, specified by the `stoplight` parameter, the
amount of buffer a flashing hand is given before the crosswalk sign changes to "Don't Walk" and the amount of time before
the stoplight turns Green that the symbol should change to Don't Walk.

Once operational, a crosswalk signal defines its status in terms of its symbol.

```
status:
  symbol: Walk
  lastTransition: 1234
```

The symbols of a crosswalk signal are either `Walk`, `DontWalk` or `FlashyHand`. Absence of a crosswalk signal
indicates that you must look both ways and hold mom's hand.

### Ambulances

```
apiVersion: safety.traffic.bwarminski.io/v1alpha1
kind: Ambulance
metadata:
  name: martys-ambulance-service-unit-223
spec:
  lightsOn: true
  crossing: elm-street-north-south
```

Ambulances have no status beyond their specification, which indicates which stoplight they're crossing and whether
their lights are on.


## Rules

Stoplights may transition from Red to Green, from Yellow to Red, from Green to Yellow.
Crosswalk signals may transition from Walk to FlashyHand to DontWalk.

Usually stoplights follow their timing patterns defined in the spec, transitioning from green to yellow after
greenLightDurationSec, Yellow to Red after yellowLightDurationSec and Red to Green after redLightDurationSec. The
only exception is when an ambulance is crossing an intersection and the light is Green, the stoplight should immediately
transition to Yellow, then Red and stay Red until the ambulance has crossed the intersection or its lights are turned
off.

A crosswalk signal should always be "DontWalk" if its stoplight is green, yellow or non-operational. Again, the ambulances
cause a problem for crosswalks, because they really shouldn't tell you to "Walk" if an emergency vehicle is barreling
through.

If an ambulance is crossing an intersection with its lights on, the related Stoplight should be Red and the crosswalk
signal should say "Don't Walk". In the event of an approaching ambulance, crosswalk signals should transition to
FlashyHand for flashingHandDurationSec and then stay DontWalk until their related stoplight's emergencyMode status is
not true.