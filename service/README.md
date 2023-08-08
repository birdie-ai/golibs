# Service

Our `service` module provides common functionality used accross services that are more
high level/service specific.

## Metrics

The library also provides common metrics for services.

#### service_build_info : gauge

This is an implemention of the idea documented [here](https://www.robustperception.io/exposing-the-software-version-to-prometheus/).

Labels:

* revision : the revision identifier for the current commit or checkout.
* goversion : Go's version used on the service.
