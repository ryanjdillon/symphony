# Changelog

## 1.0.0 (2026-03-25)


### Features

* add MultiOrchestrator and rewrite CLI for multi-workflow support ([62029d9](https://github.com/ryanjdillon/symphony/commit/62029d98c629cd94b2fbcb26a3de436d7275cda8))
* add React frontend with Vite, TypeScript, and Tailwind ([e11aa09](https://github.com/ryanjdillon/symphony/commit/e11aa099e99736a6e01ff7d46a591e74ec48f97f))
* add SSHRunner to launch agent sessions on remote hosts ([9778e0a](https://github.com/ryanjdillon/symphony/commit/9778e0a77f764d7cf940d83b356642ef524c5ba6))
* add telemetry package with OTEL meter provider and metrics ([f3ef955](https://github.com/ryanjdillon/symphony/commit/f3ef95515a936b795e2020a27a84cd015a383a0d))
* add WorkerConfig for SSH host pool configuration ([8ffb473](https://github.com/ryanjdillon/symphony/commit/8ffb473763d5ff930f0774cdd3d5389de25576e6))
* add workflow name field to LiveSession and RetryEntry ([c95b26f](https://github.com/ryanjdillon/symphony/commit/c95b26f07a634489442fcae45c34ffd21ba74d05))
* **agent:** implement agent runner interface and app-server protocol ([ae4c9bc](https://github.com/ryanjdillon/symphony/commit/ae4c9bc598f4a2f06766582e89ca44fd43b73633))
* **config:** implement workflow parser with hot-reload support ([ba81e62](https://github.com/ryanjdillon/symphony/commit/ba81e6230080f30b57b1b7182e28f4634f6edc90))
* define ToolHandler interface and result helpers ([0e2751e](https://github.com/ryanjdillon/symphony/commit/0e2751e42270f41e6485837b5851654e8da8960c))
* embed frontend in Go binary and serve at root ([c2f4139](https://github.com/ryanjdillon/symphony/commit/c2f4139bbe7279248e01ecb9fe2350c049102805))
* implement linear_graphql agent tool with mutation guard ([a028fb1](https://github.com/ryanjdillon/symphony/commit/a028fb101bb99afed4cfc488f71db1ab599997f7))
* implement SSH host pool manager with health and affinity tracking ([e367da3](https://github.com/ryanjdillon/symphony/commit/e367da3399c99ba9c55d97f9592dad36166bacad))
* **orchestrator:** add poll loop, state machine, scheduler, and CLI entrypoint ([57edfe5](https://github.com/ryanjdillon/symphony/commit/57edfe57a6db26cf26f33c61aa9de148a0dc5393))
* switch OTEL exporter to gRPC and make init conditional ([e60124f](https://github.com/ryanjdillon/symphony/commit/e60124f05f13ec58292818be009e81e0e83ba238))
* thread agent tools through orchestrator and CLI ([5b8e43c](https://github.com/ryanjdillon/symphony/commit/5b8e43ca549e86a26ca1bb5b138fa46ffead84b1))
* thread workflow name through Orchestrator and status API ([3b0ac1c](https://github.com/ryanjdillon/symphony/commit/3b0ac1c00c4edd890263798b91435189805ed9a1))
* **tracker:** add tracker interface, Issue model, and Linear GraphQL client ([ead2605](https://github.com/ryanjdillon/symphony/commit/ead260526ed3fa575506148db67c16ad3ca6e439))
* wire OTEL metrics into orchestrator and CLI ([92460ea](https://github.com/ryanjdillon/symphony/commit/92460ea7abecce6d1c2c1e2b2ad27dd441ee1fb9))
* wire SSH worker dispatch into orchestrator and CLI ([22a26e3](https://github.com/ryanjdillon/symphony/commit/22a26e3a37585fe976337b1bcb9b01e476871780))
* wire tool dispatch into app-server session protocol ([b12822c](https://github.com/ryanjdillon/symphony/commit/b12822ccd909c83d999e52f69882a6c895c8556a))
* **workspace,template,status:** add workspace lifecycle, prompt rendering, and HTTP status surface ([bb559a5](https://github.com/ryanjdillon/symphony/commit/bb559a5edb88edb56224ea3f4acc28d136b9e538))


### Bug Fixes

* correct Go base image version in Dockerfile ([718706b](https://github.com/ryanjdillon/symphony/commit/718706bb674eb8cc59d5df4ef21f2985a8f68f3a))
* correct Linear GraphQL query to use inverseRelations for blocked-by lookup ([a3c9303](https://github.com/ryanjdillon/symphony/commit/a3c930393b23a453e94d7c927d6c1484f9fd31be))
* resolve golangci-lint findings across codebase ([e6bc466](https://github.com/ryanjdillon/symphony/commit/e6bc466536264da6fb2f521ed37c9d5e6d339a02))
