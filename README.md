# OS-practice-1

Hands-on Operating Systems practice in **Go** — exploring process scheduling, threads, concurrency, and synchronization primitives.

## Overview

This project contains two standalone simulations that demonstrate core OS concepts:

| Program | File | Description |
|---|---|---|
| **Process Scheduler** | `escalonador.go` | Interactive preemptive process scheduling simulator |
| **Race Condition Demo** | `race_condition.go` | Demonstrates race conditions and mutex-based synchronization |

## Features

### Process Scheduler (`escalonador.go`)

- **Round Robin** and **Priority (Preemptive)** scheduling algorithms
- Configurable time quantum (1–10 ms)
- Interactive CLI menu to create processes, configure the scheduler, and run simulations
- Real-time progress bar during process execution
- Per-process statistics: turnaround time, wait time, and final state

### Race Condition Demo (`race_condition.go`)

- **Part 1** — Launches 5 threads that increment a shared counter *without* a mutex, showing lost updates caused by race conditions
- **Part 2** — Runs 5 threads over 2 shared resources *with* mutexes, guaranteeing mutual exclusion
- Live monitor that periodically prints resource state, ownership, and contention stats
- Final report comparing results with and without synchronization

## Prerequisites

- [Go](https://go.dev/dl/) **1.26** or later

## Getting Started

Clone the repository and navigate into it:

```bash
git clone https://github.com/zauns/OS-practice-1.git
cd OS-practice-1
```

### Running the Process Scheduler

```bash
go run escalonador.go
```

You will be prompted to:

1. Choose a scheduling algorithm (Round Robin or Priority)
2. Set the time quantum
3. Create processes with a name, priority, type (CPU-bound / I/O-bound), and CPU time
4. Start execution and view statistics

### Running the Race Condition Demo

```bash
go run race_condition.go
```

The program runs automatically in two phases and prints a final report — no user input required.

## Project Structure

```text
.
├── escalonador.go      # Process scheduling simulator
├── race_condition.go   # Race condition & mutex demo
├── go.mod              # Go module definition
├── LICENSE             # MIT License
└── README.md
```

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
