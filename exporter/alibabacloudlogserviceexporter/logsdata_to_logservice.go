// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	slsLogTimeUnixNano   = "timeUnixNano"
	slsLogSeverityNumber = "severityNumber"
	slsLogSeverityText   = "severityText"
	slsLogContent        = "content"
	slsLogAttribute      = "attribute"
	slsLogFlags          = "flags"
	slsLogResource       = "resource"
	slsLogHost           = "host"
	slsLogService        = "service"
	// shortcut for "otlp.instrumentation.library.name" "otlp.instrumentation.library.version"
	slsLogInstrumentationName    = "otlp.name"
	slsLogInstrumentationVersion = "otlp.version"
)

func logDataToLogService(ld plog.Logs) []*sls.Log {
	var slsLogs []*sls.Log
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sl := rl.ScopeLogs()
		resource := rl.Resource()
		resourceContents := resourceToLogContents(resource)
		for j := 0; j < sl.Len(); j++ {
			ils := sl.At(j)
			instrumentationLibraryContents := instrumentationScopeToLogContents(ils.Scope())
			logs := ils.LogRecords()
			for j := 0; j < logs.Len(); j++ {
				slsLog := mapLogRecordToLogService(logs.At(j), resourceContents, instrumentationLibraryContents)
				if slsLog != nil {
					slsLogs = append(slsLogs, slsLog)
				}
			}
		}
	}

	return slsLogs
}

func resourceToLogContents(resource pcommon.Resource) []*sls.LogContent {
	logContents := make([]*sls.LogContent, 3)
	attrs := resource.Attributes()
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		logContents[0] = &sls.LogContent{
			Key:   proto.String(slsLogHost),
			Value: proto.String(hostName.AsString()),
		}
	} else {
		logContents[0] = &sls.LogContent{
			Key:   proto.String(slsLogHost),
			Value: proto.String(""),
		}
	}

	if serviceName, ok := attrs.Get(conventions.AttributeServiceName); ok {
		logContents[1] = &sls.LogContent{
			Key:   proto.String(slsLogService),
			Value: proto.String(serviceName.AsString()),
		}
	} else {
		logContents[1] = &sls.LogContent{
			Key:   proto.String(slsLogService),
			Value: proto.String(""),
		}
	}

	fields := map[string]interface{}{}
	attrs.Range(func(k string, v pcommon.Value) bool {
		if k == conventions.AttributeServiceName || k == conventions.AttributeHostName {
			return true
		}
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, _ := json.Marshal(fields)
	logContents[2] = &sls.LogContent{
		Key:   proto.String(slsLogResource),
		Value: proto.String(string(attributeBuffer)),
	}

	return logContents
}

func instrumentationScopeToLogContents(instrumentationScope pcommon.InstrumentationScope) []*sls.LogContent {
	logContents := make([]*sls.LogContent, 2)
	logContents[0] = &sls.LogContent{
		Key:   proto.String(slsLogInstrumentationName),
		Value: proto.String(instrumentationScope.Name()),
	}
	logContents[1] = &sls.LogContent{
		Key:   proto.String(slsLogInstrumentationVersion),
		Value: proto.String(instrumentationScope.Version()),
	}
	return logContents
}

func mapLogRecordToLogService(lr plog.LogRecord,
	resourceContents,
	instrumentationLibraryContents []*sls.LogContent) *sls.Log {
	if lr.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	var slsLog sls.Log

	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	slsLog.Contents = make([]*sls.LogContent, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))
	contentsBuffer := make([]sls.LogContent, 0, preAllocCount)

	slsLog.Contents = append(slsLog.Contents, resourceContents...)
	slsLog.Contents = append(slsLog.Contents, instrumentationLibraryContents...)

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogTimeUnixNano),
		Value: proto.String(strconv.FormatUint(uint64(lr.Timestamp()), 10)),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogSeverityNumber),
		Value: proto.String(strconv.FormatInt(int64(lr.SeverityNumber()), 10)),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogSeverityText),
		Value: proto.String(lr.SeverityText()),
	})

	fields := map[string]interface{}{}
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, _ := json.Marshal(fields)
	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogAttribute),
		Value: proto.String(string(attributeBuffer)),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogContent),
		Value: proto.String(lr.Body().AsString()),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(slsLogFlags),
		Value: proto.String(strconv.FormatUint(uint64(lr.FlagsStruct()), 16)),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(traceIDField),
		Value: proto.String(lr.TraceID().HexString()),
	})

	contentsBuffer = append(contentsBuffer, sls.LogContent{
		Key:   proto.String(spanIDField),
		Value: proto.String(lr.SpanID().HexString()),
	})

	for i := range contentsBuffer {
		slsLog.Contents = append(slsLog.Contents, &contentsBuffer[i])
	}

	if lr.Timestamp() > 0 {
		// convert time nano to time seconds
		slsLog.Time = proto.Uint32(uint32(lr.Timestamp() / 1000000000))
	} else {
		slsLog.Time = proto.Uint32(uint32(time.Now().Unix()))
	}

	return &slsLog
}
