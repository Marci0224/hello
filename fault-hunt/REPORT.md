# Fault hunt report

Fill in one section per fault, in the order you found them. Paste real output, not a summary.

## Fault 1

- **Symptom** (what you saw first): Both Pods stayed `Pending`
- **Command** (that showed the cause): kubectl describe pod hello-7f6755ddc7-krrqn
- **Evidence** (the exact output line): Warning  FailedScheduling  75s (x5 over 26m)  default-scheduler  0/1 nodes are available: 1 Insufficient cpu. preemption: 0/1 nodes are available: 1 Preemption is not helpful for scheduling. 
- **Cause**: `resources.requests.cpu` was `64`, which means 64 whole cores. The node has far fewer, so the scheduler could not place either Pod.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers[0].resources.requests.cpu`: changed `64` to `64m`.

## Fault 2

- **Symptom** (what you saw first): Newest pod stuck in ImagePullBackOff
- **Command** (that showed the cause): kubectl describe pod hello-65ffb5d47b-gmhf5
- **Evidence** (the exact output line): Warning  Failed     54m (x5 over 57m)   kubelet            spec.containers{hello}: Failed to pull image "hello:0.1.10": failed to pull and unpack image "docker.io/library/hello:0.1.10": failed to resolve reference "docker.io/library/hello:0.1.10": pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed
- **Cause**: The image tag was set to `0.1.10`, which does not exist locally on the node (only `hello:0.1.1` is loaded) or on Docker Hub, so the kubelet could not pull the image.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers[0].image`: changed `hello:0.1.10` to `hello:0.1.1`.

## Fault 3

- **Symptom** (what you saw first): New Pod entered `CrashLoopBackOff` immediately after starting.
- **Command** (that showed the cause): kubectl describe pod hello-5bd48dd7cb-ml8kh
- **Evidence** (the exact output line):  Warning  Failed     66s (x5 over 2m28s)   kubelet            spec.containers{hello}: Error: failed to create containerd task: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: container init was OOM-killed (memory limit too low?)
- **Cause**: `resources.limits.memory` was set to `2Mi`, which is too small for the container process to initialize, causing the Linux kernel to OOM-kill it immediately.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers[0].resources.limits.memory`: changed `"2Mi"` to `64Mi`.

## Fault 4

- **Symptom** (what you saw first): New Pod entered `CrashLoopBackOff`
- **Command** (that showed the cause): kubectl logs hello-5964bfdd66-7d7nw
- **Evidence** (the exact output line): 2026/11/19 18:03:04 listen tcp: lookup tcp/abc: unknown port
- **Cause**: The `PORT` environment variable was set to `abc` instead of a valid port number, causing the application to exit immediately on startup.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers[0].env.value` (`PORT`): changed `"abc"` to `"8080"`

## Fault 5

- **Symptom** (what you saw first): New Pod was `Running` but `READY` stayed `0/1`
- **Command** (that showed the cause): kubectl describe pod hello-75ff4b7c58-bc4pb
- **Evidence** (the exact output line): Warning  Unhealthy  2s (x13 over 43s)  kubelet            spec.containers{hello}: Readiness probe failed: Get "http://10.244.0.8:8081/healthz": dial tcp 10.244.0.8:8081: connect: connection refused
- **Cause**: `readinessProbe.httpGet.port` was set to `8081` instead of `8080`, so the probe failed to connect and the Pod never became Ready.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers.readinessProbe.httpGet.port`: changed `8081` to `8080`.

## Fault 6

- **Symptom** (what you saw first): Container kept restarting (`RESTARTS` incrementing) while `Running`
- **Command** (that showed the cause): kubectl describe pod hello-75ff4b7c58-bc4pb
- **Evidence** (the exact output line): Warning  Unhealthy  4s (x4 over 34s)   kubelet            spec.containers{hello}: Liveness probe failed: Get "http://10.244.0.8:8008/healthz": dial tcp 10.244.0.8:8008: connect: connection refused
- **Cause**: `livenessProbe.httpGet.port` was set to `8008` instead of `8080`, causing Kubernetes to repeatedly kill and restart the healthy container.
- **Fix** (file and field you changed): In `deployment.yaml`, `spec.template.spec.containers.livenessProbe.httpGet.port`: changed `8008` to `8080`.

## Fault 7

- **Symptom** (what you saw first): Service `hello` had no endpoints (`ENDPOINTS: <unset>` in EndpointSlice), even though both Pods were `1/1 Running`.
- **Command** (that showed the cause): kubectl get svc,endpointslices -o wide
- **Evidence** (the exact output line): service/hello        ClusterIP   10.96.80.195   <none>        80/TCP    115m   app=hello,tier=web
- **Cause**: `spec.selector` in `service.yaml` required `tier: web` in addition to `app: hello`, which did not match the Pod labels (`app=hello`), leaving the Service with zero endpoints.
- **Fix** (file and field you changed): In `service.yaml`, `spec.selector`: removed `tier: web` so it only selects `app: hello`.

## Fault 8

- **Symptom** (what you saw first): Service `hello` had endpoints, but routed traffic to port `80` (`PORTS: 80`) instead of `8080` where the containers listen.
- **Command** (that showed the cause): kubectl get endpointslices -o wide
- **Evidence** (the exact output line): hello-7gzfl   IPv4          80      10.244.0.10,10.244.0.9   118m
- **Cause**: `spec.ports[0].targetPort` in `service.yaml` was set to `80` (or omitted, defaulting to `port: 80`), whereas the container listens on port `8080`.
- **Fix** (file and field you changed): In `service.yaml`, `spec.ports[0].targetPort`: set `targetPort: 8080`.
