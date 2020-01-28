# istio-wrapper

`istio-wrapper` is a simple wrapper application which makes it easier to run
applications within Istio that depend on the `istio-proxy` for outside network
access. It ensures that your application doesn't start until the proxy is ready,
and that the proxy is shut down when the application exits. It is best used as a
prefix to your existing Docker entrypoint. It executes any argument passed to
it, doing a simple path lookup:
```
istio-wrapper echo "hi"
istio-wrapper /bin/ls -a
```

The `istio-wrapper` won't do anything special unless you provide at least the
`ISTIO_PROXY` environment variable. This must be set to `true` for the wrapper
to take effect.

If you do provide the `ISTIO_PROXY` environment variable, `istio-wrapper`
will poll the proxy indefinitely with backoff, waiting for the proxy to report
itself as live.

All signals are passed to the underlying application. Be warned that `SIGKILL`
cannot be passed, so this can leave behind a orphaned process.

When the application exits, as long as it does so with exit code 0,
`istio-wrapper` will instruct the `istio-proxy` to shut down immediately.
