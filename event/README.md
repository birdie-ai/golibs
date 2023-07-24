# Event

Our `event` module provides common publish/subscribe functionality for events as used at Birdie.
It specializes pubsub to events, instead of general purposes messages and adds some basic schema
to the events.

## Metrics

The library also provides metrics both for publishers and subscribers. We use Prometheus for metrics.

The metrics are always sampled but are not registered by default anywhere, so if you want the metrics
you need to opt-in on them by calling [event.MustRegisterMetrics](TODO_LINK).

### Publisher

#### event_publish_duration_seconds : histogram

Measure publish duration time for each event.
Useful when debugging connectivity issues between a service and the message broker.

Labels:

* status : "ok" or "error".
* name : name of the event.

#### event_publish_total : counter

Total of published messages.

Labels:

* status : "ok" or "error".
* name : name of the event.

### Subscription

#### event_process_duration_seconds : histogram

Measure how long it took to process each event.

Labels:

* status : "ok" or "error".
* name : name of the event.

#### event_process_total : counter

Total of messages processed by a subscription.

Labels:

* status : "ok" or "error".
* name : name of the event.
