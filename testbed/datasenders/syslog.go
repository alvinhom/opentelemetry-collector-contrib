// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type SyslogWriter struct {
	testbed.DataSenderBase
	conn    net.Conn
	buf     []string
	bufSize int
	network string
}

var _ testbed.LogDataSender = (*SyslogWriter)(nil)

func NewSyslogWriter(network string, host string, port int, batchSize int) *SyslogWriter {
	f := &SyslogWriter{
		network: network,
		bufSize: batchSize,
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
	}
	return f
}

func (f *SyslogWriter) GetEndpoint() net.Addr {
	var addr net.Addr
	switch f.network {
	case "udp":
		addr, _ = net.ResolveUDPAddr(f.network, fmt.Sprintf("%s:%d", f.Host, f.Port))

	default:
		addr, _ = net.ResolveTCPAddr(f.network, fmt.Sprintf("%s:%d", f.Host, f.Port))
	}
	return addr
}

func (f *SyslogWriter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *SyslogWriter) Start() (err error) {
	f.conn, err = net.Dial(f.GetEndpoint().Network(), f.GetEndpoint().String())
	// udp not ack, can't use net.Dial to check udp server is ready, use sleep 1 second to wait udp server start
	if f.network == "udp" {
		time.Sleep(1 * time.Second)
	}
	return err
}

func (f *SyslogWriter) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).ScopeLogs().At(j)
			for k := 0; k < ills.LogRecords().Len(); k++ {
				err := f.Send(ills.LogRecords().At(k))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *SyslogWriter) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  syslog:
    protocol: rfc5424
    %s:
      listen_address: "%s"
`, f.network, f.GetEndpoint())
}
func (f *SyslogWriter) Send(lr plog.LogRecord) error {
	ts := time.Unix(int64(lr.Timestamp()/1000000000), int64(lr.Timestamp()%100000000)).Format(time.RFC3339Nano)
	sdid := strings.Builder{}
	sdid.WriteString(fmt.Sprintf("%s=\"%s\" ", "trace_id", lr.TraceID().HexString()))
	sdid.WriteString(fmt.Sprintf("%s=\"%s\" ", "span_id", lr.SpanID().HexString()))
	sdid.WriteString(fmt.Sprintf("%s=\"%d\" ", "trace_flags", lr.FlagsStruct()))
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		sdid.WriteString(fmt.Sprintf("%s=\"%s\" ", k, v.StringVal()))
		return true
	})
	msg := fmt.Sprintf("<166> %s localhost - - - [%s] %s\n", ts, sdid.String(), lr.Body().StringVal())

	f.buf = append(f.buf, msg)
	return f.SendCheck()
}

func (f *SyslogWriter) SendCheck() error {
	if len(f.buf) == f.bufSize {
		b := bytes.NewBufferString("")
		for _, v := range f.buf {
			b.WriteString(v)
		}

		_, err := f.conn.Write(b.Bytes())
		f.buf = []string{}
		if err != nil {
			return nil
		}

	}
	return nil
}

func (f *SyslogWriter) Flush() {
}

func (f *SyslogWriter) ProtocolName() string {
	return "syslog"
}
