// Copyright 2017 Louis McCormack
// Adapted by Duncan Mac-Vicar
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/codenotary/immudb/pkg/client"
	bpf "github.com/iovisor/gobpf/bcc"
	"github.com/renstrom/shortuuid"
	"google.golang.org/grpc/metadata"
	"log"
	"os"
	"os/signal"
	"time"
)

const source string = `
#include <uapi/linux/ptrace.h>

struct readline_event_t {
        u32 pid;
        char str[80];
} __attribute__((packed));

BPF_PERF_OUTPUT(readline_events);

int get_return_value(struct pt_regs *ctx) {
        struct readline_event_t event = {};
        u32 pid;
        if (!PT_REGS_RC(ctx))
                return 0;
        pid = bpf_get_current_pid_tgid();
        event.pid = pid;
        bpf_probe_read(&event.str, sizeof(event.str), (void *)PT_REGS_RC(ctx));
        readline_events.perf_submit(ctx, &event, sizeof(event));

        return 0;
}
`

type readlineEvent struct {
	Pid uint32
	Str [80]byte
}

type Entry struct {
	Pid     uint32 `json:pid`
	Command string `json:command`
}

func main() {
	m := bpf.NewModule(source, []string{})
	defer m.Close()

	readlineUretprobe, err := m.LoadUprobe("get_return_value")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load get_return_value: %s\n", err)
		os.Exit(1)
	}

	err = m.AttachUretprobe("/usr/lib64/libreadline.so", "readline", readlineUretprobe, -1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to attach return_value: %s\n", err)
		os.Exit(1)
	}

	table := bpf.NewTable(m.TableId("readline_events"), m)

	channel := make(chan []byte)

	perfMap, err := bpf.InitPerfMap(table, channel, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init perf map: %s\n", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	go func() {
		c, err := client.NewImmuClient(client.DefaultOptions())
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		// login with default username and password and storing a token
		lr, err := c.Login(ctx, []byte(`immudb`), []byte(`immudb`))
		if err != nil {
			log.Fatal(err)
		}
		// set up an authenticated context that will be required in future operations
		md := metadata.Pairs("authorization", lr.Token)
		ctx = metadata.NewOutgoingContext(context.Background(), md)

		var event readlineEvent
		for {
			data := <-channel
			err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &event)
			if err != nil {
				fmt.Printf("failed to decode received data: %s\n", err)
				continue
			}
			// Convert C string (null-terminated) to Go string
			comm := string(event.Str[:bytes.IndexByte(event.Str[:], 0)])

			entry := Entry{Pid: event.Pid, Command: comm}
			json, err := json.Marshal(entry)
			if err != nil {
				log.Fatal(err)
			}

			key := fmt.Sprintf("bash:%d:%s", time.Now().UnixNano(), shortuuid.New())
			_, err = c.Set(ctx, []byte(key), json)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	perfMap.Start()
	<-sig
	perfMap.Stop()
}
