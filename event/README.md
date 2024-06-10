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

#### event_publish_duration_seconds : histogram

Measure publish duration time for each event.
Useful when debugging connectivy issues between a service and the message broker.

Labels:

* status : "ok" or "error".
* name : name of the event.

#### event_publish_total : counter

Total of published messages.

Labels:

* status : "ok" or "error".
* name : name of the event.

### Subscription

#### event_process_msg_body_size_bytes : histogram

Measure the event's message body size in bytes.

Labels:

* status : "ok" or "error".
* name : name of the event.

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
