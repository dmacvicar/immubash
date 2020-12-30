
# BPF tracing shell commands to immudb

This self-contained program sends every entered bash command in the system to immudb.

## Introduction

The BPF ecosystem tools include in addition to the probe framework, many interesting examples, like hooking into bash readline using uprobes:

For example, with the [perf example](http://www.brendangregg.com/blog/2016-02-08/linux-ebpf-bcc-uprobes.html):

```console
# perf probe -x /bin/bash 'readline%return +0($retval):string'
Added new event:
  probe_bash:readline  (on readline%return in /bin/bash with +0($retval):string)

You can now use it in all perf tools, such as:

    perf record -e probe_bash:readline -aR sleep 1

# perf record -e probe_bash:readline -a
^C[ perf record: Woken up 1 times to write data ]
[ perf record: Captured and wrote 0.259 MB perf.data (2 samples) ]

# perf script
 bash 26239 [003] 283194.152199: probe_bash:readline: (48db60 <- 41e876) arg1="ls -l"
 bash 26239 [003] 283195.016155: probe_bash:readline: (48db60 <- 41e876) arg1="date"
# perf probe --del probe_bash:readline
Removed event: probe_bash:readline
```

or with the [bpftrace example](https://github.com/iovisor/bpftrace/blob/master/tools/bashreadline.bt):

```c
BEGIN
{
	printf("Tracing bash commands... Hit Ctrl-C to end.\n");
	printf("%-9s %-6s %s\n", "TIME", "PID", "COMMAND");
}

uretprobe:/bin/bash:readline
{
	time("%H:%M:%S  ");
	printf("%-6d %s\n", pid, str(retval));
}
```

Doing it with an embedded program and the Go SDK has advantages:

- Makes it easier to store structured data into immudb, as we have direct access to eBPF structures (eg. maps).
  For example, it would be a nice exercise to extract ``bpf_get_current_uid_gid`
  In the example we just insert it as a single entry in json, but one could use separate keys for each field.
- The program could be extended to load eBPF traces with certain format as plugins
- Better handling of the insertion lifecycle. Eg. handling every insertion as a goroutine and retry errors
- Allows embedding the eBPF logic in the same binary

So, based on https://github.com/iovisor/gobpf/blob/master/examples/bcc/bash_readline/bash_readline.go we insert the traced data into immudb in real-time.

# Caveats

* It uses the default `immudb:immudb` password
* It works with bash built with readline as shared library. If it is not the case, change `/usr/lib64/libreadline.so` to `/bin/bash`
* It is using my fork of gopf to include [this PR](https://github.com/iovisor/gobpf/pull/266)

# Building

* Make sure you have `bcc-devel`
* `go build`

# Running

```
./sudo immubash
```

(ideally should be run as a system service)

Then you should start seeing entered commands in immudb:

```
immuclient>scan bash
index:          2360
key:            bash:1609333500321974067:owRpsPUx3rF7ahM7CYdkjL
value:          {"Pid":23279,"Command":"./immuclient "}
hash:           87ae81c3b398c401f2ad9342cbb089622d842e83de4ee0a51b0a9cd671ff44a2
time:           2020-12-30 14:05:00 +0100 CET

index:          2361
key:            bash:1609333543729802108:MnKP2UAwaYWKBBrwTQDnaJ
value:          {"Pid":9268,"Command":"echo $SHELL"}
hash:           0659feb74bd3f8c62048964904a76ca9e4dca554331a435074f5fddbf0c5131a
time:           2020-12-30 14:05:43 +0100 CET
```

# License

* Apache 2.0
