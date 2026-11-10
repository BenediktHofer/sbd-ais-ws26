"""The anomaly detector as a Kafka service. YOUR CODE (Part 2, Stage C2).

The plumbing is here, the decisions (marked TODO) are yours. Contract with
the maintenance team: every reading goes through model.score(); every
reading with score >= model.threshold becomes ONE anomaly event,
model.anomaly_event(reading, score), as JSON, published to the topic
`anomalies`.

    python3 detector_service.py --name det-1
    python3 detector_service.py --name det-2          # one more instance

Decisions (see description.md, Stage C):
  - input topic `readings` (12 partitions, key = machine_id), one consumer group
    `line-guardian-detector` for all instances: Kafka gives each partition, and so
    each machine, to exactly one instance -> the model sees every reading of a
    machine, in seq order.
  - at-least-once: the offset of a reading is stored only after it was scored,
    and committed (sync) before a partition is handed over in a rebalance.
  - rebalance: the model state of machines whose partition is revoked/lost is
    dropped (model.forget), so a stale state never comes back.
  - anomalies are keyed by machine_id: all anomalies of one machine stay in order.
"""
import argparse
import json
import time

from confluent_kafka import Consumer, KafkaException, Producer

from model import BearingModel

ap = argparse.ArgumentParser()
ap.add_argument("--name", default="det-1", help="instance name (shows up in the scoreboard)")
ap.add_argument("--in-topic", default="readings", help="the topic the gateways write to")
ap.add_argument("--group", default="line-guardian-detector", help="the consumer group of all detector instances")
ap.add_argument("--out-topic", default="anomalies")
ap.add_argument("--bootstrap", default="localhost:9092")
args = ap.parse_args()

model = BearingModel(instance=args.name)
consumer = Consumer({
    "bootstrap.servers": args.bootstrap,
    "group.id": args.group,
    "client.id": args.name,
    # A NEW group starts at the end of the log: a live detector must alert on what happens now,
    # not first work through days of old runs (retention 7 days), which would break R3.
    # An EXISTING group continues at its committed offset, so a restart loses nothing.
    "auto.offset.reset": "latest",
    # at-least-once: commit only offsets of readings that were really scored
    "enable.auto.commit": True,
    "enable.auto.offset.store": False,      # we call store_offsets() after model.score()
    "auto.commit.interval.ms": 1000,
    # only the partitions that really have to move are revoked, the others keep running (and keep their state)
    "partition.assignment.strategy": "cooperative-sticky",
    "session.timeout.ms": 10000,            # a crashed instance is replaced after 10 s
})
producer = Producer({
    "bootstrap.servers": args.bootstrap,
    "client.id": f"{args.name}-anomalies",
    "acks": "all",                          # an alert must not get lost
    "enable.idempotence": True,             # no duplicates / reordering through retries
    "linger.ms": 0,                         # alerts are rare and urgent: send at once (R3)
})

partition_of = {}                           # machine_id -> partition it was read from


def _drop_state(partitions, why):
    gone = {p.partition for p in partitions}
    machines = [m for m, p in partition_of.items() if p in gone]
    for m in machines:
        model.forget(m)                     # another instance takes over; our state would be stale later
        del partition_of[m]
    print(f"[{args.name}] {why} partitions {sorted(gone)} -> forgot {len(machines)} machines", flush=True)


def on_assign(c, partitions):
    print(f"[{args.name}] assigned partitions {sorted(p.partition for p in partitions)}", flush=True)


def on_revoke(c, partitions):
    producer.flush(10)                      # anomalies of everything we scored are out ...
    try:
        c.commit(asynchronous=False)        # ... before the next owner continues after our last offset
    except KafkaException:
        pass                                # nothing stored yet
    _drop_state(partitions, "revoked")


def on_lost(c, partitions):
    _drop_state(partitions, "lost")         # we were kicked out; no commit possible any more


consumer.subscribe([args.in_topic], on_assign=on_assign, on_revoke=on_revoke, on_lost=on_lost)
print(f"[{args.name}] {args.in_topic} -> {args.out_topic}, group {args.group} (Ctrl+C to stop)", flush=True)

readings = anomalies = 0
report_at = time.time() + 5
try:
    while True:
        msg = consumer.poll(1.0)
        if msg is None:
            continue
        if msg.error():
            print(f"[{args.name}] {msg.error()}")
            continue
        reading = json.loads(msg.value())
        partition_of[reading["machine_id"]] = msg.partition()
        score = model.score(reading)
        readings += 1
        if score is not None and score >= model.threshold:
            anomalies += 1
            event = model.anomaly_event(reading, score)
            producer.produce(args.out_topic, key=reading["machine_id"],   # per-machine order of alerts
                             value=json.dumps(event))
        consumer.store_offsets(msg)         # scored -> may be committed
        producer.poll(0)
        if time.time() >= report_at:
            lag = time.time() - reading["ts"]
            print(f"[{args.name}] {readings:>9,} readings  {anomalies:>6,} anomalies  "
                  f"missing {model.missing}  out-of-order {model.out_of_order}  "
                  f"machines {len(model.machines())}  delay {lag:4.2f} s", flush=True)
            report_at += 5
except KeyboardInterrupt:
    pass
finally:
    producer.flush(10)
    consumer.close()                        # commits stored offsets and leaves the group -> fast rebalance
    print(f"[{args.name}] stopped after {readings:,} readings, {anomalies:,} anomalies")
