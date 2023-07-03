# Event

Our `event` module provides common publish/subscribe functionality for events as used at Birdie.
It specializes pubsub to events, instead of general purposes messages and adds some basic schema
to the events.

## Metrics

The library also provides metrics both for publishers and subscribers. We use prometheus for metrics.
The metrics are always sampled but are not registered by default anywhere, so if you want the metrics
you need to opt-in on them by calling [event.RegisterMetrics](TODO_LINK).

Here we document the metrics generated.

### Publisher

#### event_published_duration_seconds : histogram

Measure publish duration time for each event.
Useful when debugging connectivy issues between a service and the message broker.

Labels:

* status : "ack" or "nack".
* name : name of the event.

#### event_published_total : counter

Total of published messages.

Labels:

* status : "ack" or "nack".
* name : name of the event.

### Subscription

#### event_handled_duration_seconds : histogram

Measure how long it took to handle each event.

Labels:

* status : "ack" or "nack".
* name : name of the event.

#### event_handled_total : counter

Total of messages handled by a subscription.

Labels:

* status : "ack" or "nack".
* name : name of the event.
