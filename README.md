# Sentinel Grid

**A modular, hardware-agnostic C4ISR software platform for distributed sensing, operational data processing, edge continuity, and tactical situational awareness.**

> **Project status:** experimental prototype / research platform

Sentinel Grid is a multi-service software ecosystem that models the foundations of a distributed command-and-control platform. It combines Go services, a Python orchestration layer, a development-only battlefield simulator, and a browser-based tactical dashboard.

The repository currently focuses on **infrastructure and algorithmic prototypes**. It does not provide certified military software, production-ready autonomy, real sensor drivers, operational targeting, or a deployable defense system.

## Why Sentinel Grid?

The project explores how independent services can work together when a system must:

- move prioritized data through unreliable or congested networks;
- maintain state across distributed and intermittently connected nodes;
- monitor service health and resource usage;
- transform and validate data across polyglot components;
- process simulated optical, thermal, acoustic, signal, OSINT, and satellite data;
- expose a unified tactical picture to operators; and
- continue operating locally when an edge node loses connectivity.

The original internal module names are retained in the source tree as historical codenames. They are mapped to English descriptions throughout this document.

## Current implementation at a glance

| Area | Current state |
| --- | --- |
| Implemented services | 19 Go services plus the Python gateway/orchestrator |
| Working scope | Infrastructure services, analysis engines, APIs, simulator, dashboard |
| Go tests | Passing across all Go modules at the time of review |
| Python tests | The simulator test suite currently has an import/collection issue in the checked-out tree |
| Production readiness | Not production-ready |
| External sensor hardware | Not implemented |
| ML model inference | Mostly simulated or algorithmic; no model weights are shipped |

## Architecture

```text
                         ┌──────────────────────────┐
                         │      Sentinel Grid        │
                         │   Python gateway layer    │
                         │  registry / routing / API │
                         └────────────┬─────────────┘
                                      │
             ┌────────────────────────┼────────────────────────┐
             │                        │                        │
      ┌──────▼──────┐          ┌──────▼──────┐          ┌──────▼──────┐
      │ Data plane  │          │ State plane │          │ Observability│
      │ broker/QoS  │          │ KV/WAL/edge │          │ health/alerts │
      └──────┬──────┘          └──────┬──────┘          └─────────────┘
             │                        │
      ┌──────▼────────────────────────▼──────┐
      │ Analysis services and tactical UI     │
      │ detection / thermal / acoustic / OSINT│
      │ satellite change / tracking / dashboard│
      └───────────────────────────────────────┘
```

The planned architecture is organized into five conceptual phases:

1. **ULAK / Backbone** — communication, state, orchestration, health, edge, and UI foundations.
2. **GÖRGÜ / Perception** — sensor and intelligence processing.
3. **BUYRUK / Decision** — prioritization, planning, resource allocation, and human oversight concepts.
4. **KOVAN / Autonomy** — navigation, coordination, and vehicle behavior concepts.
5. **KIN / Resilience** — defensive security, communications resilience, and electronic protection concepts.

Only part of Phase 1 and a subset of the planned analysis services are implemented in code. The five-phase, 60-module architecture is a roadmap, not a claim that all modules are complete.

## Implemented components

### Core infrastructure

| Directory | Codename | What is implemented |
| --- | --- | --- |
| `butun/` | BÜTÜN | FastAPI gateway, service registry/discovery, load balancing, circuit breakers, lifecycle orchestration, dependency graph, resource monitoring, scheduling, scaling concepts, and rate-limiting middleware. |
| `bag/` | BAĞ | Kafka-like in-process broker, topics/partitions, consumer groups, priority queues, retry/backoff transport, gRPC and HTTP status interfaces. |
| `kutuk/` | KÜTÜK | Sharded in-memory key-value store, TTL, namespaces, watch support, WAL, snapshots, and a RAFT-oriented state engine. |
| `dirlik/` | DİRLİK | CPU/RAM/disk/GC metrics, sliding-window anomaly detection, HTTP/TCP/gRPC probes, health checks, and alert delivery. |
| `bagdas/` | BAĞDAŞ | JSON, MessagePack, and CBOR conversion, schema registry/versioning, validation, adapters, and composable data pipelines. |
| `akim/` | AKIM | Network topology, congestion-aware routing, alternate routes, flow management, QoS classes, five load-balancing strategies, and link monitoring. |
| `yurt/` | YURT | Sharded local KV storage, vector clocks, dirty tracking, TTL/tombstones, offline request buffering, replay, conflict policies, and partition/split-brain state handling. |
| `yuzey/` | YÜZEY | HTTP tactical dashboard, module health polling, entity/layer/event stores, SSE event streaming, filters, and a browser canvas view. |

### Analysis and operational prototypes

| Directory | Focus | Current behavior |
| --- | --- | --- |
| `im/` | Identity | Local identity registry, token management, and PKI-oriented prototypes. |
| `yayin/` | Broadcast | Compression, encryption, broadcast handling, and forward-error-correction prototypes. |
| `durak/` | QoS queueing | Priority queue and scheduling APIs. |
| `tutum/` | Resource/energy | Battery and resource tracking, prediction, and alert APIs. |
| `tani/` | Object detection | Bounding boxes, confidence scores, NMS, object classes, and tracking. The detector explicitly generates simulated detections; it does not load a YOLO model. |
| `koz/` | Thermal analysis | Thermal signatures, temperature history, vehicle profiles, state transitions, and pruning. Inputs are structured entities, not camera/IR frames. |
| `sezi/` | Texture analysis | Texture and camouflage-oriented feature/detection algorithms over structured inputs. |
| `yanki/` | Acoustic localization | Acoustic event and localization service prototype. |
| `duyum/` | OSINT analysis | Source collection and rule-based semantic analysis, entity extraction, geographic clustering, threat classification, confidence scoring, and reports. |
| `baskala/` | Satellite change detection | Synthetic/metadata-based imagery, image comparison, change detection, SAR/optical fusion, and timeline/threat APIs. |
| `sozgec/` | Spectrum analysis | Synthetic signal generation, FFT, PSD, peak detection, band extraction, and signal classification. It does not connect to an SDR. |
| `izlek/` | Track prediction | Kinematic prediction, maneuver confidence, Kalman-style state handling, and interception-coordinate calculations for test data. |

### Development tools

- `simulasyon/` contains a Python scenario runner, synthetic battlefield/sensor/communications generators, module clients, console reporting, and a web entry point.
- `tanitim/` contains a standalone static project presentation page.
- `plan.md` contains the original Turkish architecture and roadmap document.
- `docker-compose.yml` describes a multi-container deployment for the services.
- `Makefile` provides build, test, run, clean, and Docker helper targets.

## Repository layout

```text
.
├── README.md                 # Project overview and limitations
├── Makefile                  # Build, test, and Docker commands
├── docker-compose.yml        # Multi-service development deployment
├── .env.dev                  # Development defaults
├── .env.prod                 # Production-oriented defaults (review before use)
├── butun/                    # Python gateway/orchestrator
├── simulasyon/               # Python simulator and synthetic data generators
├── tanitim/                  # Static presentation page
├── bag/ kutuk/ dirlik/ ...   # Independent Go services
├── bin/                      # Local build output directory
└── plan.md                   # Architecture roadmap and module inventory
```

Each Go service is an independent Go module with its own `go.mod`, `cmd/<service>/main.go`, internal packages, tests, and usually a Dockerfile. There is no root `go.mod` or root Python package that turns the entire repository into one installable library.

## Requirements

- Go toolchain compatible with the module declarations (the modules currently declare Go 1.26).
- Python 3.9+ for the current development environment; Python 3.10+ is recommended for new work.
- `pip` and virtual environments.
- Docker and Docker Compose for containerized runs.
- GNU Make for the convenience commands.

## Quick start

### Run an individual Go service

```bash
cd bag
go test ./...
go run ./cmd/bag
```

Repeat the same pattern for `kutuk`, `dirlik`, `bagdas`, `akim`, `yurt`, `yuzey`, or another Go module. Check each service's `internal/config` package and local README, where available, for ports and environment variables.

### Run the Python gateway

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r butun/requirements.txt
cd butun
python -m src.main
```

The gateway normally listens on `http://localhost:8000` and exposes interactive API documentation at `/docs`.

### Run the simulator

```bash
source .venv/bin/activate
pip install -r simulasyon/requirements.txt
cd simulasyon
OTUKEN_ENV=development python -m src.cli list-scenarios
OTUKEN_ENV=development python -m src.cli generate --type sensor --count 5
OTUKEN_ENV=development python -m src.cli run --scenario temel --ticks 10 --interval 0.2
```

The simulator is intentionally restricted to development-like environments. With `--connect`, it attempts to call running services such as the gateway, broker, and state store.

### Start the container stack

```bash
docker compose up --build
```

The compose file launches the service set described in its configuration, including the gateway, data broker, state store, monitoring, edge, tactical UI, and additional analysis services. The compose configuration should be treated as a development baseline until its health checks, dependency ordering, secrets, persistence, and service contracts have been hardened.

## Common commands

```bash
make build-all       # Build all Go binaries into bin/
make test-go         # Run Go tests for all declared Go modules
make test-python     # Run the BÜTÜN Python tests
make test-sim        # Run simulator tests
make test-all        # Run all configured test targets
make docker-build    # Build configured images
make docker-up       # Start Docker Compose
make docker-down     # Stop Docker Compose
make clean           # Remove local build/cache output
```

## Service ports

The following are the defaults represented by the checked-in configuration files. Individual service configuration code is authoritative if a value differs.

| Service | HTTP/status | gRPC or secondary |
| --- | ---: | ---: |
| `butun` | `8000` | — |
| `bag` | `9091` | `9090` |
| `kutuk` | `9093` | `9092` |
| `dirlik` | `9095` | `9094` |
| `bagdas` | `9097` | `9096` is reserved in Compose |
| `akim` | `9099` | `9098` |
| `yurt` | `9101` | `9100` |
| `yuzey` | `9103` | — |
| `im` | `9105` | `9104` |
| `yayin` | `9107` | `9106` |
| `durak` | `9109` | `9108` |
| `tutum` | `9111` | `9110` |
| `tani` | `9113` | `9112` |
| `koz` | `9115` | `9114` |
| `sezi` | `9116` | `9117` |
| `yanki` | `9118` | `9119` |
| `duyum` | `9120` | `9121` |
| `baskala` | `9122` | `9123` |
| `sozgec` | `9124` | `9125` |
| `izlek` | `9126` | `9127` |

## API overview

Most services expose a small HTTP API with a health endpoint and module-specific operations. The most complete endpoint inventories are in the local READMEs:

- [`butun/README.md`](butun/README.md) — gateway, service registry, orchestration, resources, dependency graph, and circuit breakers.
- [`bag/README.md`](bag/README.md) — topics, publishing, and status.
- [`kutuk/README.md`](kutuk/README.md) — KV operations, namespaces, scans, and RAFT status.
- [`dirlik/README.md`](dirlik/README.md) — metrics, probes, and alerts.
- [`bagdas/README.md`](bagdas/README.md) — conversion, validation, schemas, adapters, and pipelines.
- [`akim/README.md`](akim/README.md) — topology, routes, flows, balancing, and congestion.
- [`yurt/README.md`](yurt/README.md) — local store, synchronization, proxy, and partition APIs.
- [`yuzey/README.md`](yuzey/README.md) — dashboard, entities, layers, events, and SSE.

For services without a local README, inspect `internal/server/server.go` before integrating. API contracts are prototypes and are not versioned or guaranteed stable yet.

## Testing and verification

The Go test suites were run during the repository review with:

```bash
for mod in bag kutuk dirlik bagdas akim yurt yuzey im yayin durak tutum tani koz sezi yanki duyum baskala sozgec izlek ortam; do
  (cd "$mod" && go test ./... -count=1 -timeout 60s) || exit 1
```

All of those Go module test commands passed in the reviewed workspace. The Python command below was also attempted:

```bash
python3 -m pytest butun/tests simulasyon/tests -q --tb=short
```

The combined Python collection currently fails before running the simulator test because `simulasyon/tests/test_generators.py` is imported as `tests.test_generators` and collides with the other `tests` package. This is documented here deliberately so the repository status is reproducible rather than overstated.

## Known limitations and incomplete work

This section is part of the project contract. The repository should not be presented as a completed operational defense platform.

### Architecture and integration

- The repository has many independent Go modules instead of a single workspace or versioned dependency graph.
- Service-to-service integration is partial. Several services expose APIs but are not wired into a complete end-to-end data path.
- API schemas, event contracts, compatibility rules, authentication requirements, and error semantics are not centrally versioned.
- Docker Compose contains a broad service inventory, but readiness, persistence, secrets, dependency ordering, and all health checks require further validation.
- The simulator launcher exists, but its run/stop helper path and process-management behavior still need to be validated across platforms.
- There is no CI workflow, release process, generated API documentation, migration system, or automated compatibility check.

### Data and algorithms

- Several modules use synthetic inputs, heuristics, random values, or simplified mathematical models.
- `tani` does not load YOLO weights or perform image inference; its detector explicitly simulates detections.
- `koz` consumes structured entities and derives thermal signatures; it does not ingest real IR frames or calibrate a thermal sensor.
- `sozgec` synthesizes signal samples before FFT analysis; it does not interface with SDR hardware or demodulate live traffic.
- `duyum` includes rule-based keyword/entity extraction rather than a production NLP/LLM pipeline.
- `baskala` supports synthetic imagery and comparison workflows; it does not provide a validated satellite-data ingestion pipeline.
- `izlek` uses a simplified kinematic predictor and must not be treated as a validated fire-control or interception solution.
- Claimed technologies in `plan.md` and the presentation page—including advanced deep learning, swarm autonomy, stealth, post-quantum cryptography, and offensive cyber capabilities—are roadmap concepts unless backed by corresponding implementation files and tests.

### Security and operations

- Development and production environment files are examples, not secure deployment profiles.
- Secrets management, certificate rotation, key custody, audit logging, authorization policy, secure defaults, and threat modeling are incomplete.
- The current Compose network is a bridge network and is not an air-gapped or hardened deployment.
- No operational safety case, independent security review, formal verification, or compliance assessment is included.
- Performance numbers in module documentation are local benchmark observations, not service-level guarantees or field measurements.

### Testing and quality

- Unit tests cover many internal components, but integration, contract, load, fault-injection, recovery, and security testing are incomplete.
- The simulator Python test collection issue must be fixed before claiming a green repository-wide test run.
- There are no shipped model datasets, model cards, calibration reports, reproducibility manifests, or accuracy benchmarks for perception modules.
- Several local READMEs and source comments are Turkish while the repository-level documentation is English; terminology and generated docs still need normalization.

## Suggested next milestones

1. Add a root Go workspace and a single, reproducible Python project configuration.
2. Repair the simulator entrypoint and the duplicate `tests` package import problem.
3. Define versioned protobuf/JSON schemas and compatibility tests for every service boundary.
4. Add CI for formatting, static analysis, unit tests, integration tests, container builds, and vulnerability scanning.
5. Replace synthetic analysis paths with explicit adapter interfaces and documented test fixtures.
6. Add persistence and recovery tests for the state, edge, broker, and orchestration services.
7. Establish authentication, authorization, certificate management, secret handling, audit trails, and secure production defaults.
8. Add model/data governance before introducing real perception or decision-support models.
9. Keep all high-consequence automation behind explicit simulation boundaries and human approval workflows.

## Responsible use

Sentinel Grid is intended for software architecture research, simulation, education, and defensive systems engineering. It should be used with synthetic or authorized data and in controlled environments. It is not a substitute for certified systems, trained operators, legal review, safety engineering, or human judgment.

## Contributing

Contributions are welcome in infrastructure reliability, test coverage, documentation, observability, interoperability, simulation quality, and defensive security. Please include:

- the affected service and its interface impact;
- tests or reproducible steps;
- whether behavior is real, simulated, or a placeholder;
- configuration and migration notes; and
- security or operational considerations for any network-facing change.

Before opening a pull request, run the relevant module tests and `gofmt` on changed Go files. Do not commit credentials, private keys, production data, generated binaries, or local environment secrets.

## License

No license file is currently included. Until a license is added, the repository should be treated as **all rights reserved** and should not be redistributed or reused as open-source software.
