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

### Subscription
