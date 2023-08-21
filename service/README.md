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

#### http_requests_in_flight_total : gauge

Amount of in flight HTTP requests on the service.

#### http_request_duration_seconds : histogram

HTTP request duration distribution

Labels:

* code    : HTTP status code, like `200`
* method  : HTTP method of the request, like `POST`
* handler : Path of the request, like `/request/path`

#### http_requests_total : count

Counts all HTTP requests handled by a service.

Labels:

* code    : HTTP status code, like `200`
* method  : HTTP method of the request, like `POST`
* handler : Path of the request, like `/request/path`
