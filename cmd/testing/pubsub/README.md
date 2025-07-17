# PubSub Testing

Very simple tooling to check ordered event delivery.
Export variables:

```sh
export GOOGLE_PROJECT=<project>
export TOPIC_NAME=<topic>
```

The topic will be created if it doesn't exist, same goes for the subscription which will have the same name.
Use an unique topic name or else the results of your test may not make much sense (someone else might be
using the same topic/subscription).

Now run:

```sh
go run . subscriber
```

And then:

```sh
go run . publisher
```

Resources won't be automatically deleted (except subscription that have an expiration policy of one day), remember to delete them.
