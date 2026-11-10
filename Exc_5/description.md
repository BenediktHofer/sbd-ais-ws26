# Exercise 5: Part 2 *Line Guardian*, my solution

I read `model.py` first. The model is **stateful per machine**: it learns a
machine's normal vibration from its first 100 readings (`WARMUP`) and then
compares a sliding window of the last 50 readings against that baseline.
It needs **every** reading of a machine, **in `seq` order**: a lower `seq`
is counted as `out_of_order` and ignored, a gap is counted as `missing`.
Every `score()` call costs **3 ms**, so one instance can score at most
~333 readings/s.

---

## Stage A: Today's system

### A1: Small plant (20 machines × 10 Hz = 200 readings/s), 90 s

Terminal 1, then terminal 2 (instead of `Ctrl+C` on the plant I used
`--duration 90`):

```bash
python3 baseline_detector.py
python3 plant.py --sink postgres --machines 20 --duration 90
```

Plant (terminal 2):

```
run 1794328155: 20 machines x 10 Hz = 200 readings/s -> postgres (Ctrl+C to stop)
  t=    5s      1,020 readings sent (204/s)
  ...
  t=   40s  fault starts on M-006 (repaired in 45 s)
  t=   40s      8,020 readings sent (200/s)  faulty: M-006
  ...
  t=   85s     17,020 readings sent (200/s)  faulty: M-006
run 1794328155: sent 18,000 readings in 91 s
```

Detector (terminal 1, the 324 `ANOMALY M-006 ...` lines left out):

```
[baseline] polling table readings for id > 0 (Ctrl+C to stop)
[baseline] read      428  backlog      25 rows  delay   0.2 s  missing     66  anomalies 0
[baseline] read    1,264  backlog      35 rows  delay   0.3 s  missing    205  anomalies 0
[baseline] read    2,114  backlog      40 rows  delay   0.2 s  missing    365  anomalies 0
[baseline] read    2,933  backlog      45 rows  delay   0.3 s  missing    526  anomalies 0
[baseline] read    3,806  backlog       5 rows  delay   0.1 s  missing    668  anomalies 0
[baseline] read    4,612  backlog      27 rows  delay   0.2 s  missing    852  anomalies 0
[baseline] read    5,462  backlog      15 rows  delay   0.2 s  missing  1,057  anomalies 0
[baseline] read    6,231  backlog      21 rows  delay   0.2 s  missing  1,248  anomalies 0
[baseline] read    7,093  backlog      15 rows  delay   0.2 s  missing  1,376  anomalies 0
[baseline] read    7,980  backlog      16 rows  delay   0.2 s  missing  1,509  anomalies 0
[baseline] read    8,865  backlog      25 rows  delay   0.2 s  missing  1,654  anomalies 34
[baseline] read    9,678  backlog      35 rows  delay   0.3 s  missing  1,831  anomalies 75
[baseline] read   10,492  backlog       6 rows  delay   0.1 s  missing  1,987  anomalies 112
[baseline] read   11,360  backlog      39 rows  delay   0.3 s  missing  2,169  anomalies 152
[baseline] read   12,110  backlog      45 rows  delay   0.2 s  missing  2,354  anomalies 187
[baseline] read   12,966  backlog      33 rows  delay   0.2 s  missing  2,538  anomalies 226
[baseline] read   13,792  backlog      20 rows  delay   0.2 s  missing  2,712  anomalies 266
[baseline] read   14,564  backlog      30 rows  delay   0.2 s  missing  2,890  anomalies 301
[baseline] read   15,019  backlog       0 rows  delay   0.0 s  missing  2,961  anomalies 324
[baseline] stopped: read 15,019 readings, last id 18,000, 2,961 readings missing, 324 anomalies
```

After the backlog was 0 I stopped the detector and counted the table:

```bash
docker exec -it postgres psql -U postgres -d factory \
  -c "SELECT count(*) FROM readings WHERE run = (SELECT max(run) FROM readings);"
```

```
 count
-------
 18000
(1 row)
```

**Answers A1**

- **The detector keeps up** (backlog stays below 50 rows, delay ~0.2 s), but
  it read only **15,019 of 18,000** readings: **~3,000 readings (16 %)
  are missing**, and `missing` keeps growing for the whole run.
- **Yes, the missing readings are in the table.** All 18,000 rows are
  there. The detector just never saw them.
- **Why:** `PostgresSink` has 4 gateway threads. Each one INSERTs a batch,
  then sleeps 0–120 ms ("gateway work + network") and only then COMMITs.
  The `id` (BIGSERIAL) is assigned at **INSERT** time, but other sessions
  see the row only after the **COMMIT**. So ids become visible in a
  different order than they were assigned.
- **The timeline from the README:**
  1. Gateway 1 inserts ids **100–114** and commits 100 ms later.
     Gateway 2 inserts ids **115–129** and commits at once.
  2. The detector polls in between: `SELECT ... WHERE id > 99`. It sees only
     **115–129** (committed). 100–114 exist but are invisible (not
     committed yet).
  3. The detector remembers **`last_id = 129`**.
  4. Gateway 1 commits 100–114. The next poll asks `WHERE id > 129`, so
     **100–114 are skipped forever.** For the model this is a gap in the
     `seq` of those machines, so `missing` grows.
- `id > last_id` is only a correct cursor if ids become visible in id
  order. With several concurrent writers they do not.
  Consequence: the model gets an incomplete stream and **R2 is violated**,
  even though the detector is fast enough.

### A2: Full plant (60 machines × 10 Hz = 600 readings/s), ~60 s

```bash
python3 baseline_detector.py --quiet
python3 plant.py --sink postgres --duration 60
```

Plant:

```
run 1794328275: 60 machines x 10 Hz = 600 readings/s -> postgres (Ctrl+C to stop)
  t=    5s      3,060 readings sent (611/s)
  ...
  t=   40s  fault starts on M-029 (repaired in 45 s)
  ...
run 1794328275: sent 36,000 readings in 61 s
```

Detector (watched **while the plant was running**, stopped right after the plant):

```
[baseline] polling table readings for id > 18000 (Ctrl+C to stop)
[baseline] read      674  backlog     840 rows  delay   1.5 s  missing     30  anomalies 0
[baseline] read    3,344  backlog   4,186 rows  delay   7.0 s  missing     45  anomalies 0
[baseline] read    5,344  backlog   6,564 rows  delay  11.0 s  missing     60  anomalies 0
[baseline] read    7,344  backlog   9,066 rows  delay  15.2 s  missing     60  anomalies 0
[baseline] read    9,344  backlog  11,640 rows  delay  19.4 s  missing     60  anomalies 0
[baseline] read   11,344  backlog  14,155 rows  delay  23.7 s  missing     60  anomalies 0
[baseline] read   13,344  backlog  16,880 rows  delay  28.2 s  missing     60  anomalies 0
[baseline] read   15,344  backlog  19,549 rows  delay  32.7 s  missing     60  anomalies 0
[baseline] stopped: read 15,344 readings, last id 34,014, 60 readings missing, 0 anomalies
```

```
 count
-------
 36000
```

**Answers A2**

- **`delay` grows linearly** (1.5 s → 32.7 s in 60 s) and **`backlog` grows
  without limit** (+~2,500 rows per report). The detector is limited by the
  model: 3 ms per reading gives at most **~333 readings/s**, while the plant
  produces 600 readings/s. Every second ~270 readings pile up in the table.
  It is a single, sequential poller, so there is no way to scale it out.
- **The fault on M-029 (t = 40 s) was never detected** (`anomalies 0`). The
  detector was ~30 s behind and never reached those rows before I stopped
  it. Even if it had, the alert would have come 30 s or more late.
  **R3 (≤ 5 s) is violated, and it gets worse every second.**
- **`missing` almost stops growing** (stuck at 60). That looks paradoxical
  but is the same effect as in A1 from the other side. The commit/visibility
  race only hurts when the detector reads at the **head** of the table,
  where rows are not yet committed. A detector that is 30 s behind only
  reads rows that were committed long ago, so it sees no gaps. The 60
  missing readings all come from the first seconds, while it was still at
  the head.
- Conclusion: **fast enough means incomplete (A1), complete means too late
  (A2)**. And this is only 600 readings/s, not 100,000.

---

## Stage B: Decision record

**Context.** Bearing-fault detection with a stateful per-machine model
(3 ms/reading). Lab: 600 readings/s, target: 100,000 readings/s. R1 scale,
R2 complete and ordered per machine, R3 alert ≤ 5 s (p95), R4 extra
consumers (archive, dashboard) without side effects and without load on
PostgreSQL. PostgreSQL stays the system of record for the machine registry.

**Option 1: keep PostgreSQL, poll better** (e.g. N pollers, each for a
subset of machines, `WHERE machine_id IN (...) AND id > last_id`).

| | Assessment |
|---|---|
| R1 | ⚠️ At risk. N pollers fix the CPU limit of the model, but at 100,000 inserts/s PostgreSQL has to absorb ~8.6 billion rows/day (indexes, WAL, vacuum) **plus** ~300+ pollers each querying several times per second. The database becomes the bottleneck. |
| R2 | ❌ Not with `id > last_id`: the commit/visibility race of A1 stays, per poller. It can be fixed with a per-machine cursor (`WHERE machine_id = … AND seq > last_seq`), but that is more queries and needs an index on `(machine_id, seq)`. |
| R3 | ⚠️ Poll interval + batch commit delay + query time. Achievable at small scale, hard at 100k/s under load. |
| R4 | ❌ **Violated by design.** Each new consumer (archive, dashboard) is another poller on the same database, which means more load and more risk for everyone, including the machine registry. |

**Option 2: the gateways write every reading to Kafka** (topic `readings`,
key = `machine_id`). Detector instances form one consumer group, and
archive and dashboard are separate groups.

| | Assessment |
|---|---|
| R1 | ✅ Scales with partitions. More partitions plus more detector instances in the same group gives linear scale-out. Kafka itself: 100,000 msg/s × ~130 B ≈ 13 MB/s, easy for a small cluster (e.g. 3 brokers, RF 3). |
| R2 | ✅ Key = `machine_id` → same partition → strictly ordered, and one partition is read by exactly one instance of the group, so every reading of a machine reaches the same model, in order. The producer uses `acks=all` + idempotence (no loss, no duplicates/reorder on retry). Offsets are per partition, and there is no visibility race like with BIGSERIAL. |
| R3 | ✅ Push-like streaming (long poll), latency in the ms range as long as there are enough instances. Measured: p95 0.09 s. |
| R4 | ✅ Each consumer is its own group with its own offsets. Reading deletes nothing, so a new group (even one that replays from `earliest`) does not change, slow down or break the detector, and touches PostgreSQL not at all. |
| Risks | Operating Kafka (the group already runs it). Rebalances move machines between instances, so the model must re-warm (~15 s blind time for those machines). Partition count is hard to change later (keys get remapped). At-least-once can give duplicates (handled with an idempotent model and commit-on-revoke, see Stage C). |

**Own idea (option 3): CDC/outbox.** Keep writing to PostgreSQL and stream
the changes to Kafka (Debezium). This keeps the existing gateways, but
PostgreSQL still carries the full 100k inserts/s. It is useful as a
migration step, not as the end state.

**Decision: option 2 (Kafka).** It is the only option that meets R1–R4 at
the same time. In particular, R4 cannot be met by a database that every
consumer has to poll. PostgreSQL stays the system of record for the
**machine registry** (`machines`), which is low-volume, queried and updated
data, exactly what a database is good at. If needed, the training-data
archive can write from Kafka into its own store.

**Numbers (detector instances, 3 ms per reading → 333 readings/s per instance):**

| Load | CPU need | Minimum | My choice |
|---|---|---|---|
| 600 readings/s (lab) | 600 × 3 ms = 1.8 instance-seconds/s | **2** (90 % busy) | **3** (60 % busy, and survives the loss of one instance only just: 2 × 333 = 666 > 600, see experiment C2) |
| 100,000 readings/s | 100,000 × 3 ms = 300 instance-seconds/s | **300** | **~430** (70 % target load + failover headroom) |

- Partitions must be ≥ instances (one partition → max one consumer per group).
  Lab: **12 partitions** (divisible by 1, 2, 3, 4, 6, 12, so there is room for
  up to 12 instances). Rollout: plan **~600 partitions** from the start,
  because adding partitions later changes `hash(key) mod n` and moves machines.
- Better than more instances: a faster model (batch/vectorised inference)
  cuts the 3 ms per call. The model is a given here, though.

**Tipping point.** The small office (1 reading/min ≈ 0.017 readings/s, one
consumer) is perfectly served by PostgreSQL: that is 4–5 orders of
magnitude below the limit. PostgreSQL alone stops being enough when **one**
of the following is true:
1. One poller can no longer keep up. Here that is already at **~300
   readings/s** because of the model cost, and in general at a few thousand
   rows/s for insert + poll.
2. **Several independent consumers** need the same stream (R4).
3. **Ordering/completeness with several writers** matters (the A1 race).

So, roughly: from a few hundred to ~1,000 events/s, or as soon as more
than one or two services need to react to the stream. Below that,
PostgreSQL is simpler and fine.

---

## Stage C: Build the core (Kafka)

### C1: Topics and gateways

I created the topics **before** starting the plant, because a topic that is
auto-created by the first write gets only 1 partition:

```bash
docker exec -it kafka kafka-topics.sh --bootstrap-server localhost:9092 \
  --create --topic readings --partitions 12 --replication-factor 1
docker exec -it kafka kafka-topics.sh --bootstrap-server localhost:9092 \
  --create --topic anomalies --partitions 3 --replication-factor 1
docker exec -it kafka kafka-topics.sh --bootstrap-server localhost:9092 \
  --describe --topic readings
```

```
Created topic readings.
Created topic anomalies.
Topic: readings	TopicId: dLLCrOxCRtm5_jy8pJsT-A	PartitionCount: 12	ReplicationFactor: 1	Configs: min.insync.replicas=1
	Topic: readings	Partition: 0	Leader: 1	Replicas: 1	Isr: 1	Elr: 	LastKnownElr:
	...
	Topic: readings	Partition: 11	Leader: 1	Replicas: 1	Isr: 1	Elr: 	LastKnownElr:
```

`KafkaSink` in [`guardian/plant.py`](guardian/plant.py):

```python
class KafkaSink:
    TOPIC = "readings"      # created beforehand with 12 partitions, not by the first write

    def __init__(self, args):
        from confluent_kafka import Producer
        self.errors = []
        self.producer = Producer({
            "bootstrap.servers": args.bootstrap,
            "client.id": "plant-gateways",
            "acks": "all",                  # a write counts only once the broker has stored it
            "enable.idempotence": True,     # retries do not create duplicates or change the order
            "linger.ms": 5,                 # batch for max 5 ms instead of one request per reading
            "compression.type": "lz4",
        })

    def send(self, reading):
        # key = machine_id: same machine -> same partition -> same detector, in seq order (R2)
        while True:
            try:
                self.producer.produce(self.TOPIC, key=reading["machine_id"], value=json.dumps(reading),
                                      on_delivery=self._on_delivery)
                return
            except BufferError:             # local queue full: let librdkafka send, then retry (never drop)
                self.producer.poll(0.05)

    def tick(self):
        self.producer.poll(0)               # delivery callbacks; no flush() per reading (too slow)
        if self.errors:
            raise RuntimeError(f"Kafka delivery failed: {self.errors[0]}")
```

Decisions:
- **Key `machine_id`**: `hash(key) mod 12` always maps a machine to the same
  partition, so all its readings stay in `seq` order and reach one detector
  instance (R2).
- **`acks=all` + `enable.idempotence`**: no reading lost, and no duplicate or
  reordering through producer retries (R2).
- **`linger.ms=5`, `lz4`, no `flush()` per reading**: high throughput at
  only +5 ms latency (R1/R3). The plant never printed `behind schedule`.

### C2: Detector service

Full code: [`guardian/detector_service.py`](guardian/detector_service.py).
The decisions, i.e. the former `TODO`s:

| Question | Decision | Why |
|---|---|---|
| Input topic | `readings` | the gateways' topic (12 partitions) |
| Group | `line-guardian-detector`, the **same** for all instances | Kafka splits the 12 partitions among the instances; each machine is handled by exactly one model instance |
| `auto.offset.reset` | `latest` | For a **new** group: start at the end. A live detector must alert on *now*. With `earliest` a new group would first replay up to 7 days of old runs (retention), which means huge delay and violates R3. An **existing** group continues at its committed offset anyway, so a restart loses nothing. |
| Commits | `enable.auto.offset.store=False` + `store_offsets(msg)` after `model.score()`, auto-commit every 1 s | at-least-once: only offsets of readings that were really scored get committed |
| Rebalance | `cooperative-sticky`, `on_revoke`: `producer.flush()` + sync `commit()` + `model.forget()` for the machines of the revoked partitions; `on_lost`: `forget()` | Only the partitions that have to move are moved. The next owner continues exactly after our last scored reading, so there are no duplicates. State of a machine we hand over is dropped, so if the partition comes back later we do not continue with a stale state (which would show up as `missing`). |
| Anomaly key | `machine_id` | all anomalies of one machine in order, in one partition of `anomalies` |
| Producer | `acks=all`, idempotence, `linger.ms=0` | alerts are rare and urgent: no batching delay, no loss |
| Instances | **3** (`det-1`, `det-2`, `det-3`) | Stage B: 600 × 3 ms = 1.8 → min 2, 3 for headroom |

### C3: Run 1: 3 instances, 60 machines × 10 Hz, 200 s (3 faults)

```bash
python3 scoreboard.py --reset
python3 detector_service.py --name det-1     # terminal 1
python3 detector_service.py --name det-2     # terminal 2
python3 detector_service.py --name det-3     # terminal 3
python3 plant.py --sink kafka --duration 200 # terminal 4
```

```
detector_health emptied
```

Partition assignment as the instances joined (cooperative rebalancing: only
the partitions that move are revoked):

```
[det-1] assigned partitions [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]
[det-1] revoked partitions [0, 1, 2, 3, 4, 5] -> forgot 0 machines      # det-2 joins
[det-2] assigned partitions [0, 1, 2, 3, 4, 5]
[det-2] revoked partitions [0, 1] -> forgot 0 machines                  # det-3 joins
[det-1] revoked partitions [6, 7] -> forgot 0 machines
[det-3] assigned partitions [0, 1, 6, 7]
```

Result: det-1 = 8–11, det-2 = 2–5, det-3 = 0, 1, 6, 7 (4 partitions each).

Plant:

```
run 1794328440: 60 machines x 10 Hz = 600 readings/s -> kafka (Ctrl+C to stop)
  t=    5s      3,060 readings sent (612/s)
  ...
  t=   40s  fault starts on M-029 (repaired in 45 s)
  ...
  t=  100s  fault starts on M-017 (repaired in 45 s)
  ...
  t=  160s  fault starts on M-020 (repaired in 45 s)
  ...
run 1794328440: sent 120,000 readings in 200 s
```

Consumer group during the run (t ≈ 110 s):

```bash
docker exec -it kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group line-guardian-detector
```

```
GROUP                  TOPIC     PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG   CLIENT-ID
line-guardian-detector readings  8          6618            6666            48    det-1
line-guardian-detector readings  11         3308            3333            25    det-1
line-guardian-detector readings  10         6618            6666            48    det-1
line-guardian-detector readings  9          4412            4444            32    det-1
line-guardian-detector readings  4          5510            5555            45    det-2
line-guardian-detector readings  3          4408            4444            36    det-2
line-guardian-detector readings  2          3306            3333            27    det-2
line-guardian-detector readings  5          6612            6666            54    det-2
line-guardian-detector readings  1          4408            4444            36    det-3
line-guardian-detector readings  7          9913            9999            86    det-3
line-guardian-detector readings  6          6612            6666            54    det-3
line-guardian-detector readings  0          4404            4444            40    det-3
```

The lag is tiny: it is only what accumulates within the 1 s auto-commit
interval. The log-end offsets also show that **`hash(key) mod 12` does not
spread 60 machines evenly**: partition 7 holds 9 machines, partition 11
only 3. So det-3 has 23 machines and det-2 has 18. More partitions per
instance would even this out.

Detector output (end of run):

```
[det-1]    37,385 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 0.07 s
[det-2]    35,227 readings     367 anomalies  missing 0  out-of-order 0  machines 18  delay 0.01 s
[det-3]    45,932 readings     688 anomalies  missing 0  out-of-order 0  machines 23  delay 0.02 s
```

Scoreboard:

```bash
python3 scoreboard.py
```

```
Run 1794328440: started 17:34:00 (3.4 min ago), 60 machines x 10 Hz, 3 faults so far

Detection  (topic anomalies: 1,058 anomaly events, models ['v1'])
   #  machine   start  detected   delay
   1  M-029       40s       yes   7.9 s
   2  M-017      100s       yes  12.2 s
   3  M-020      160s       yes  11.9 s

  faults detected: 3/3, mean delay 10.7 s; false alarms (episodes on healthy machines): 0
  latency reading -> anomaly event: p50 0.05 s, p95 0.09 s, max 0.13 s
  duplicate anomaly events (same anomaly_id): 0

Detector instances  (model telemetry, this run)
  det-1          pid 36899   v1  readings    37,330  missing       0  out-of-order      0  machines   19  alive
  det-2          pid 37003   v1  readings    35,364  missing       0  out-of-order      0  machines   18  alive
  det-3          pid 37107   v1  readings    45,184  missing       0  out-of-order      0  machines   23  alive

Requirements (measurable part)
  [PASS] R2 complete and ordered: 0 readings missing, 0 out of order
  [PASS] R3 fresh: p95 latency < 5 s
  [PASS] detection: all faults found, at most 2 false alarms
```

**Everything passes.** Compared with Stage A:
- **0 missing, 0 out of order** (A1: ~3,000 missing), because the key keeps
  each machine in one partition and that partition is read by one instance.
- **p95 latency 0.09 s** (A2: 30 s+ and growing), because 3 instances share
  the load (each ~200 readings/s, ~60 % of 333/s).
- **Detection delay ~8–12 s is not pipeline latency.** It is the model: the
  drift (+0.05 per second) has to grow enough that the 50-reading window
  mean exceeds the threshold. The pipeline adds only ~0.1 s.

### C4: Run 2: failure experiment, crash of `det-3` (explaining a FAIL)

I wanted to see what a rebalance does to the stateful model. I started
another run with the same 3 instances, then **killed det-3 hard**
(`kill -9`, simulating a crash, so no clean `close()`) at t = 110 s, and
started it again at t = 140 s:

```bash
python3 plant.py --sink kafka --duration 170      # terminal 4
kill -9 <pid of det-3>                            # at t=110 s
docker exec -it kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group line-guardian-detector       # at t=125 s
python3 detector_service.py --name det-3          # at t=140 s, terminal 3
python3 scoreboard.py
```

Group 15 s after the crash: det-3's partitions 0, 1, 6, 7 have moved to
det-1 and det-2, and the lag is much higher than before:

```
GROUP                  TOPIC     PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG   CLIENT-ID
line-guardian-detector readings  1          12856           13088           232   det-1
line-guardian-detector readings  8          19284           19632           348   det-1
line-guardian-detector readings  7          28926           29448           522   det-1
line-guardian-detector readings  11         9642            9816            174   det-1
line-guardian-detector readings  10         19284           19632           348   det-1
line-guardian-detector readings  9          12856           13088           232   det-1
line-guardian-detector readings  4          16135           16360           225   det-2
line-guardian-detector readings  3          12912           13088           176   det-2
line-guardian-detector readings  2          9684            9816            132   det-2
line-guardian-detector readings  6          19366           19632           266   det-2
line-guardian-detector readings  5          19368           19632           264   det-2
line-guardian-detector readings  0          12912           13088           176   det-2
```

det-1 log: it takes over partitions 1 and 7 (32 machines), the delay
rises, and when det-3 comes back it gives them up again and forgets their
state:

```
[det-1]    60,534 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 0.08 s
[det-1] assigned partitions [1, 7]
[det-1]    61,791 readings       0 anomalies  missing 0  out-of-order 0  machines 32  delay 6.49 s
[det-1]    63,101 readings       0 anomalies  missing 0  out-of-order 0  machines 32  delay 5.99 s
[det-1]    64,401 readings       0 anomalies  missing 0  out-of-order 0  machines 32  delay 6.89 s
[det-1]    65,731 readings       0 anomalies  missing 0  out-of-order 0  machines 32  delay 7.69 s
[det-1] revoked partitions [1, 7] -> forgot 13 machines
[det-1]    66,990 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 6.59 s
[det-1]    68,268 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 3.79 s
[det-1]    70,854 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 3.29 s
[det-1]    72,135 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 1.49 s
[det-1]    73,362 readings       0 anomalies  missing 0  out-of-order 0  machines 19  delay 0.09 s
```

The restarted det-3 takes over partitions 0, 1, 2, 7 and catches up:

```
[det-3] assigned partitions [0, 1, 2, 7]
[det-3]       339 readings       0 anomalies  missing 0  out-of-order 0  machines 7  delay 13.10 s
[det-3]     1,631 readings       0 anomalies  missing 0  out-of-order 0  machines 20  delay 11.79 s
[det-3]     2,906 readings       0 anomalies  missing 0  out-of-order 0  machines 20  delay 7.50 s
[det-3]     4,202 readings       0 anomalies  missing 0  out-of-order 0  machines 20  delay 6.10 s
[det-3]     5,479 readings       0 anomalies  missing 0  out-of-order 0  machines 20  delay 4.69 s
[det-3]     6,758 readings       0 anomalies  missing 0  out-of-order 0  machines 20  delay 3.29 s
```

Scoreboard:

```
Run 1794328663: started 17:37:43 (2.9 min ago), 60 machines x 10 Hz, 3 faults so far

Detection  (topic anomalies: 715 anomaly events, models ['v1'])
   #  machine   start  detected   delay
   1  M-029       40s       yes   7.9 s
   2  M-005      100s       yes   7.3 s
   3  M-059      160s  (just started, not scored yet)

  faults detected: 2/2, mean delay 7.6 s; false alarms (episodes on healthy machines): 0
  latency reading -> anomaly event: p50 0.09 s, p95 5.20 s, max 6.37 s
  duplicate anomaly events (same anomaly_id): 0

Detector instances  (model telemetry, this run)
  det-1          pid 36899   v1  readings    72,660  missing       0  out-of-order      0  machines   19  alive
  det-2          pid 37003   v1  readings    69,427  missing       0  out-of-order      0  machines   21  alive
  det-3          pid 37107   v1  readings    70,280  missing       0  out-of-order      0  machines   23  silent for 67 s
  det-3          pid 47015   v1  readings     6,370  missing       0  out-of-order      0  machines   20  alive

Requirements (measurable part)
  [PASS] R2 complete and ordered: 0 readings missing, 0 out of order
  [FAIL] R3 fresh: p95 latency < 5 s
  [PASS] detection: all faults found, at most 2 false alarms
```

**Explanation of the FAIL (R3)**

1. **Crash → 10 s nobody reads.** After `kill -9`, det-3 does not leave the
   group cleanly. The group coordinator only notices when its heartbeats
   stop for `session.timeout.ms = 10 s`. During that time, partitions
   0, 1, 6, 7 (23 machines, ~230 readings/s) are not read at all, and
   ~2,300 readings pile up.
2. **Two instances are almost at their limit.** After the rebalance, det-1
   and det-2 have to carry 600 readings/s with a capacity of
   2 × 333 = 666/s. det-1 got 32 machines (~320/s against 333/s), so it
   could hardly reduce the backlog, and the delay even grew (6.5 → 7.7 s).
   The fault on M-005 was still running (t = 100–145 s), and its anomaly
   events from this phase were published several seconds late, which pushed
   **p95 to 5.2 s** (max 6.4 s). The first alert at 7.3 s came before the
   crash, which is why detection itself still passes.
3. **Recovery.** Only when det-3 came back at t = 140 s did the capacity rise
   to 1,000/s, and the delay dropped back to 0.09 s within ~25 s.

**What stayed correct (and why):**
- **R2: 0 missing, 0 out of order, 0 duplicate anomalies**, even through two
  rebalances. det-3's last committed offset was at most ~1 s old, so the new
  owner reprocessed at most ~1 s of readings. But it started those machines
  with a **fresh** model state (`forget()` / new process), so for the model
  this was a clean new start, not a gap or out-of-order. When det-3
  returned, det-1/det-2 committed synchronously in `on_revoke` and forgot
  those machines (`forgot 13 machines`, `forgot 7 machines`). If they get
  them back later, there is no stale state either.
- **Price of a rebalance for a stateful model:** every machine that moves
  needs 100 readings warm-up + 50 window = **~15 s in which it cannot raise
  an alert**. If a fault starts on such a machine in that window, the alert
  comes later.

**What I would change for production:**
- **N+1 sizing:** plan instances so that the remaining ones can carry the
  full load after one crash at < 80 % utilisation (lab: **4 instances**;
  rollout: ~430 → plus a failure margin).
- Lower `session.timeout.ms` (e.g. 6 s, the broker minimum), with a
  trade-off against false "dead" detections.
- Use more partitions than instances (e.g. 24–48 in the lab) so a failed
  instance's machines are spread over *all* survivors instead of a few,
  which also evens out the uneven key distribution.
- Optional: keep the model state outside the process (state store /
  checkpoint topic keyed by machine) so a new owner does not have to
  re-warm. That is what Kafka Streams/Flink do with changelog topics.

### C5: Stop everything

```bash
# Ctrl+C in each detector terminal
docker exec -it kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group line-guardian-detector
```

```
[det-1] stopped after 73,629 readings, 0 anomalies
[det-2] stopped after 70,456 readings, 648 anomalies
[det-3] stopped after 6,865 readings, 0 anomalies

Consumer group 'line-guardian-detector' has no active members.
```

---

## Summary

| | PostgreSQL + polling (Stage A) | Kafka, 3 detector instances (Stage C) |
|---|---|---|
| R1 scale | 1 poller, max ~333 readings/s; backlog grows at 600/s | scales with partitions/instances (12 partitions → up to 12 instances) |
| R2 complete/ordered | ~3,000 of 18,000 readings missing (commit-visibility race) | 0 missing, 0 out of order |
| R3 fresh | delay 32 s and growing; fault not detected | p95 0.09 s (5.2 s after a crash, explained above) |
| R4 independent consumers | every consumer is another poller on the DB | every consumer is its own consumer group, no DB load |

Files changed: `guardian/plant.py` (`KafkaSink`), `guardian/detector_service.py`.
Topic commands: see C1. All raw outputs: `logs/`.
