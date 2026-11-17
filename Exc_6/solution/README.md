## Benedikt Hofer

# Part 1

> one paragraph, 5–8 sentences. Which
> option do you choose for this scenario, and why? What does it cost, and how
> would you notice in production before it hurts? Which option would you
> choose if your team owned the payment path?

I choose option 4, log-based CDC with Debezium, because it is the only option that meets R1 and R2: the payment path stays unchanged, gets no extra queries, and every committed payment is read from the WAL in commit order.

Polling misses rows that commit out of `id` order and adds query load, while dual write and outbox both change the certified payment path.

The cost is the replication slot: PostgreSQL keeps all WAL that Debezium has not read yet, so a long connector outage can fill the disk and stop the database.

I would notice this early by monitoring the retained WAL per slot in `pg_replication_slots` and the connector status, with an alert well before the disk is full.

If my team owned the payment path, I would choose the outbox pattern, because it is just as correct and gives us an explicit event contract instead of exposing the internal table layout.

# Part 2

## Run 1

> Predict: do all three velocity instances share the work, and does R4
> pass? Write down your guess and your reason, then run the routine (steps
> 2–5; there is nothing to stop yet). Also look in the Kafka UI: how many partitions does the topic have, and what is
> the key of an event?

**Prediction:** I expect that only one velocity instance does the work and R4 fails, because nobody told Debezium how many partitions to create, so the topic probably has only one.

**Kafka UI:** the topic `fraud.public.transactions` has **1 partition**, and the key of an event is the primary key `id` of the transaction (`{"payload": {"id": 1}}`).

**Agents (`python3 agents.py`):**

```
[vel-3] assigned partitions []
[vel-2] assigned partitions []
[vel-1] assigned partitions [0]
[ano-1] assigned partitions [0]
...
[vel-1]    49,820 transactions      5 alerts   39,192 cardholders  lag   81.1 s  out-of-order 0
[ano-1]   132,302 transactions      6 alerts   73,321 cardholders  lag    0.8 s  out-of-order 0
```

**Scoreboard (`python3 scoreboard.py`, after ~2 minutes):**

```
Run 1794929813: started 16:36:53, 135,185 transactions in 135 s (1,001/s), 14 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   1        1     1,101
  velocity-agent           fraud.public.transactions                   1        3    84,652

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          66354 anomaly         134,113       73,801             0      0.7       5s ago
  vel-1          66355 velocity         51,776       40,352             0     84.2       1s ago

  velocity model (card testing)
    R3  frauds caught          2/7  (29%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  71.1 s / 84.9 s   FAIL  (p95 <= 5 s after the commit)
```

**Why:** within a consumer group, each partition is read by exactly one instance, so with one partition only `vel-1` gets payments, and `vel-2` and `vel-3` stay idle.

`vel-1` handles only about 380 of the 1,000 payments per second, so its lag grows by about 3 s every 5 s and R4 fails.

R3 fails as a consequence: most frauds are still waiting in the backlog and have not been scored yet, while the anomaly model (0.3 ms per payment) keeps up and passes.

## Run 2

> Predict: now all three instances work. Will the velocity model catch
> ≥ 90 % of the card-testing frauds (R3)? Think of the key you saw in Run 1.
> Then save the file and repeat the whole routine, from step 1: stop the
> agents and the card processor, run connector.py again, restart both, wait,
> scoreboard. You know it worked when the agents print [vel-1] assigned partitions [0, 1], [vel-2] … [2, 3], [vel-3] … [4, 5].

**Change in `fraud-connector.json`:**

```json
"topic.creation.default.partitions": "6",
"topic.creation.default.replication.factor": "1"
```

**Prediction:** I expect that R4 now passes, because all three instances share the work, but R3 fails, because the key is still the payment `id`, so the payments of one cardholder are spread over all partitions.

**Agents (`python3 agents.py`):**

```
[vel-3] assigned partitions [4, 5]
[vel-1] assigned partitions [0, 1]
[ano-1] assigned partitions [0, 1, 2, 3, 4, 5]
[vel-2] assigned partitions [2, 3]
...
[vel-1]    43,128 transactions      1 alerts   34,951 cardholders  lag    1.0 s  out-of-order 0
[ano-1]   132,195 transactions      6 alerts   73,263 cardholders  lag    0.9 s  out-of-order 0
[vel-2]    43,279 transactions      0 alerts   35,151 cardholders  lag    0.9 s  out-of-order 0
[vel-3]    43,077 transactions      2 alerts   34,934 cardholders  lag    0.9 s  out-of-order 0
```

**Scoreboard (`python3 scoreboard.py`, after ~2 minutes):**

```
Run 1794930890: started 16:54:50, 130,181 transactions in 130 s (1,001/s), 14 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     1,914
  velocity-agent           fraud.public.transactions                   6        3     1,996

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          84188 anomaly         133,834       73,703             0      1.0       0s ago
  vel-1          84189 velocity         43,171       34,976             0      0.9       2s ago
  vel-2          84190 velocity         43,440       35,255             0      0.9       2s ago
  vel-3          84191 velocity         43,199       35,010             0      0.9       2s ago

  velocity model (card testing)
    R3  frauds caught          2/6  (33%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.9 s / 1.0 s   PASS  (p95 <= 5 s after the commit)
```

**Why:** with 6 partitions, every velocity instance gets two of them and handles about a third of the payments, so the lag stays below 1 s and R4 passes.

R3 fails because the key is still the payment `id`: the partition is `hash(id) mod 6`, so the 12 test payments of one stolen card land on random partitions, and each instance sees only about 4 of them, below the threshold of 8.

The anomaly model still passes only because it runs a single instance that reads all six partitions; the key must be the cardholder (`user_id`), the entity whose state the model keeps.

## Run 3

> Predict: what will the key of an event look like in the Kafka UI, and
> what changes on the scoreboard? Then save the file and repeat the whole
> routine, from step 1, as in Run 2.

**Change in `fraud-connector.json`:**

```json
"message.key.columns": "public.transactions:user_id"
```

**Prediction:** I expect the key to be `{"user_id": ...}` instead of `{"id": ...}`, so all payments of one cardholder land in the same partition and the velocity model catches its frauds, while R4 still passes.

**Key of an event (Kafka UI / console consumer):**

```
{"schema":{..., "name":"fraud.public.transactions.Key"},"payload":{"user_id":142388}}
```

**Agents (`python3 agents.py`):**

```
[vel-1] assigned partitions [0, 1]
[ano-1] assigned partitions [0, 1, 2, 3, 4, 5]
[vel-3] assigned partitions [4, 5]
[vel-2] assigned partitions [2, 3]
...
[vel-1]    43,540 transactions     12 alerts   24,378 cardholders  lag    0.8 s  out-of-order 0
[vel-3]    42,882 transactions     22 alerts   24,020 cardholders  lag    1.0 s  out-of-order 0
[vel-2]    43,326 transactions      0 alerts   24,392 cardholders  lag    0.9 s  out-of-order 0
[ano-1]   132,383 transactions      6 alerts   73,484 cardholders  lag    0.9 s  out-of-order 0
```

**Scoreboard (`python3 scoreboard.py`, after ~2 minutes):**

```
Run 1794931439: started 17:03:59, 135,185 transactions in 135 s (1,001/s), 14 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     4,304
  velocity-agent           fraud.public.transactions                   6        3     4,327

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          93373 anomaly         134,486       74,009             0      1.0       2s ago
  vel-1          93374 velocity         43,617       24,399             0      0.9       4s ago
  vel-2          93375 velocity         43,431       24,421             0      0.9       4s ago
  vel-3          93376 velocity         42,968       24,045             0      0.9       4s ago

  velocity model (card testing)
    R3  frauds caught          6/6  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  1.0 s / 1.1 s   PASS  (p95 <= 5 s after the commit)
```

**Why:** the key is now the cardholder, so `hash(user_id) mod 6` sends all payments of one card to the same partition, where one instance reads them in commit order.

That instance sees all 12 test payments in its 60-s window and alerts at the 8th, so R3 passes, and with 6 partitions the work is still shared, so R4 passes too.

`vel-2` raised no alert in this run simply because none of the stolen cards hashed to its partitions.

# Closing answer

> Debezium chose two defaults that you had to change. What would have
> happened in production if nobody had checked them, and how could the team
> have noticed?

With the default of 1 partition, the velocity model could never scale: extra instances would stay idle and the lag would grow until alerts arrive minutes too late.

With the default key `id`, scaling would look fine, but every instance would see only part of each cardholder's payments, and card-testing frauds would silently go undetected.

Neither default crashes or logs an error, so without a scoreboard the team would only find out from chargebacks weeks later.

They could have noticed by monitoring consumer lag and the partitions assigned per instance (idle members), and by checking the alert rate against known frauds, for example with injected test frauds like our scoreboard does.