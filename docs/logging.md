# Logging

Relayer is using the [zap](https://pkg.go.dev/go.uber.org/zap) logging framework.

Zap is a production quality logging library that is performant and highly configurable.
Zap produces structured logging that can be easily machine-parsed,
so that people can easily use tooling to filter through logs,
or logs can easily be sent to log aggregation tooling.

## Conventions

### Instantiation

Structs should hold a reference to their own `*zap.Logger` instance, as an unexported field named `log`.
Callers should provide the logger instance as the first argument to a `NewFoo` method, setting any extra fields with the `With` method externally.
For example:

```go
path := "/tmp/foo"
foo := NewFoo(log.With(zap.String("path", path)), path)
```

### Usage

Log messages should begin with an uppercase letter and should not end with any punctuation.
Log messages should be constant string literals; dynamic content is to be set as log fields.
Names of log fields should be all lowercase `snake_case` format.

```go
// Do this:
x.log.Debug("Updated height", zap.Int64("height", height))

// Don't do this:
x.log.Debug(fmt.Sprintf("Updated height to %d.", height))
```

Errors are intended to be logged with `zap.Error(err)`.
Error values should only be logged when they are not returned.

```go
// Do this:
for _, x := range xs {
  if err := x.Validate(); err != nil {
    y.log.Info("Skipping x that failed validation", zap.String("id", x.ID), zap.Error(err))
    continue
  }
}

// Don't do this:
if err := x.Validate(); err != nil {
  y.log.Info("X failed validation", zap.String("id", x.ID), zap.Error(err))
  return err // Bad!
  // Returning an error that has already been logged may cause the same error
  // to be logged many times.
}
```

#### Log levels

Debug level messages are off by default.
Debug messages should be used sparingly;
they are typically only intended for consumption by Relayer developers when actively debugging an issue.

Info level messages and higher are on by default.
They may contain any general information useful to an operator.
It should be safe to discard all info level messages if an operator so desires.

Warn level messages should be used to indicate a problem that may worsen without operator intervention.

Error level messages should only be used to indicate a serious problem that cannot automatically recover.
Error level messages should be reserved for events that are worthy of paging an engineer.

Do not use Fatal or Panic level messages.
