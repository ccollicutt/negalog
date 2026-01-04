# Test Log Files

## linux_syslog.log

Source: [LogHub - Linux Dataset](https://github.com/logpai/loghub/tree/master/Linux)

This is the `Linux_2k.log` file from the LogHub project, containing 2000 log entries collected from `/var/log/messages` on a Linux server over 260+ days.

**Format:** Standard syslog (BSD format)
```
Jun 14 15:16:01 combo sshd(pam_unix)[19939]: authentication failure; ...
```

**Timestamp pattern:** `^\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`
**Go layout:** `Jan 2 15:04:05`

**Citation:**
```
Jieming Zhu, Shilin He, Pinjia He, Jinyang Liu, Michael R. Lyu.
Loghub: A Large Collection of System Log Datasets for AI-driven Log Analytics.
IEEE International Symposium on Software Reliability Engineering (ISSRE), 2023.
```

## heartbeat.log

Synthetic log file for testing periodic detection (heartbeat gaps).

**Format:** Syslog (same as linux_syslog.log)
```
Jan 15 10:00:00 app-server healthcheck[1001]: HEARTBEAT service=api status=healthy latency=12ms
```

**Test scenarios:**
- Regular heartbeats every 5 minutes
- Gap 1: 10:15 to 10:35 (20 min gap - should trigger alert with 7m max_gap)
- Gap 2: 11:00 to 11:20 (20 min gap - should trigger alert)
- Only 19 heartbeats total (tests min_occurrences=50 threshold)

## sample.log

A minimal synthetic log file for unit testing basic functionality.
