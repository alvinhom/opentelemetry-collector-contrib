// Copyright 2019, OpenTelemetry Authors
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

package translator

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestCauseWithExceptions(t *testing.T) {
	errorMsg := "this is a test"
	attributeMap := make(map[string]interface{})

	span := constructExceptionServerSpan(attributeMap, ptrace.StatusCodeError)
	span.Status().SetMessage(errorMsg)

	event1 := span.Events().AppendEmpty()
	event1.SetName(ExceptionEventName)
	event1.Attributes().UpsertString(conventions.AttributeExceptionType, "java.lang.IllegalStateException")
	event1.Attributes().UpsertString(conventions.AttributeExceptionMessage, "bad state")
	event1.Attributes().UpsertString(conventions.AttributeExceptionStacktrace, `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument`)

	event2 := span.Events().AppendEmpty()
	event2.SetName(ExceptionEventName)
	event2.Attributes().UpsertString(conventions.AttributeExceptionType, "EmptyError")

	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	res.Attributes().UpsertString(conventions.AttributeTelemetrySDKLanguage, "java")
	isError, isFault, isThrottle, filteredResult, cause := makeCause(span, filtered, res)

	assert.True(t, isFault)
	assert.False(t, isError)
	assert.False(t, isThrottle)
	assert.Equal(t, filtered, filteredResult)
	assert.NotNil(t, cause)
	assert.Len(t, cause.Exceptions, 3)
	assert.NotEmpty(t, cause.Exceptions[0].ID)
	assert.Equal(t, "java.lang.IllegalStateException", *cause.Exceptions[0].Type)
	assert.Equal(t, "bad state", *cause.Exceptions[0].Message)
	assert.Len(t, cause.Exceptions[0].Stack, 3)
	assert.Equal(t, cause.Exceptions[1].ID, cause.Exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *cause.Exceptions[1].Type)
	assert.NotEmpty(t, cause.Exceptions[2].ID)
	assert.Equal(t, "EmptyError", *cause.Exceptions[2].Type)
	assert.Empty(t, cause.Exceptions[2].Message)
}

func TestCauseExceptionWithoutError(t *testing.T) {
	var nonErrorStatusCodes = []ptrace.StatusCode{ptrace.StatusCodeUnset, ptrace.StatusCodeOk}

	for _, element := range nonErrorStatusCodes {
		ExceptionWithoutErrorHelper(t, element)
	}
}

func ExceptionWithoutErrorHelper(t *testing.T, statusCode ptrace.StatusCode) {
	errorMsg := "this is a test"

	var exceptionStack = `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument`

	attributeMap := make(map[string]interface{})

	span := constructExceptionServerSpan(attributeMap, statusCode)
	span.Status().SetMessage(errorMsg)

	event1 := span.Events().AppendEmpty()
	event1.SetName(ExceptionEventName)
	event1.Attributes().UpsertString(conventions.AttributeExceptionType, "java.lang.IllegalStateException")
	event1.Attributes().UpsertString(conventions.AttributeExceptionMessage, "bad state")
	event1.Attributes().UpsertString(conventions.AttributeExceptionStacktrace, exceptionStack)

	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	res.Attributes().UpsertString(conventions.AttributeTelemetrySDKLanguage, "java")
	isError, isFault, isThrottle, filteredResult, cause := makeCause(span, filtered, res)

	assert.False(t, isFault)
	assert.False(t, isError)
	assert.False(t, isThrottle)
	assert.Equal(t, filtered, filteredResult)
	assert.NotNil(t, cause)
	assert.Len(t, cause.Exceptions, 2)
	assert.NotEmpty(t, cause.Exceptions[0].ID)
	assert.Equal(t, "java.lang.IllegalStateException", *cause.Exceptions[0].Type)
	assert.Equal(t, "bad state", *cause.Exceptions[0].Message)
	assert.Len(t, cause.Exceptions[0].Stack, 3)
}

func TestEventWithoutExceptionWithoutError(t *testing.T) {
	var nonErrorStatusCodes = []ptrace.StatusCode{ptrace.StatusCodeUnset, ptrace.StatusCodeOk}

	for _, element := range nonErrorStatusCodes {
		EventWithoutExceptionWithoutErrorHelper(t, element)
	}
}

func EventWithoutExceptionWithoutErrorHelper(t *testing.T, statusCode ptrace.StatusCode) {
	errorMsg := "this is a test"

	attributeMap := make(map[string]interface{})

	span := constructExceptionServerSpan(attributeMap, statusCode)
	span.Status().SetMessage(errorMsg)

	event1 := span.Events().AppendEmpty()
	event1.SetName("NotException")
	attributes := pcommon.NewMap()
	attributes.InsertString(conventions.AttributeHTTPMethod, "Post")
	attributes.CopyTo(event1.Attributes())

	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	res.Attributes().InsertString(conventions.AttributeTelemetrySDKLanguage, "java")
	isError, isFault, isThrottle, filteredResult, cause := makeCause(span, filtered, res)

	assert.False(t, isFault)
	assert.False(t, isError)
	assert.False(t, isThrottle)
	assert.Equal(t, filtered, filteredResult)
	assert.Nil(t, cause)
}

func TestCauseWithStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[conventions.AttributeHTTPStatusCode] = 500
	span := constructExceptionServerSpan(attributes, ptrace.StatusCodeError)
	span.Status().SetMessage(errorMsg)
	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	isError, isFault, isThrottle, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isFault)
	assert.False(t, isError)
	assert.False(t, isThrottle)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
	w := testWriters.borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func TestCauseWithHttpStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[conventions.AttributeHTTPStatusCode] = 500
	attributes["http.status_text"] = errorMsg
	span := constructExceptionServerSpan(attributes, ptrace.StatusCodeError)
	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	isError, isFault, isThrottle, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isFault)
	assert.False(t, isError)
	assert.False(t, isThrottle)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
	w := testWriters.borrow()
	if err := w.Encode(cause); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, errorMsg))
}

func TestCauseWithZeroStatusMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[conventions.AttributeHTTPStatusCode] = 500
	attributes["http.status_text"] = errorMsg

	span := constructExceptionServerSpan(attributes, ptrace.StatusCodeUnset)
	filtered, _ := makeHTTP(span)
	// Status is used to determine whether an error or not.
	// This span illustrates incorrect instrumentation,
	// marking a success status with an error http status code, and status wins.
	// We do not expect to see such spans in practice.
	res := pcommon.NewResource()
	isError, isFault, isThrottle, filtered, cause := makeCause(span, filtered, res)

	assert.False(t, isError)
	assert.False(t, isFault)
	assert.False(t, isThrottle)
	assert.NotNil(t, filtered)
	assert.Nil(t, cause)
}

func TestCauseWithClientErrorMessage(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[conventions.AttributeHTTPStatusCode] = 499
	attributes["http.status_text"] = errorMsg

	span := constructExceptionServerSpan(attributes, ptrace.StatusCodeError)
	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	isError, isFault, isThrottle, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
	assert.False(t, isThrottle)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
}

func TestCauseWithThrottled(t *testing.T) {
	errorMsg := "this is a test"
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "POST"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/widgets"
	attributes[conventions.AttributeHTTPStatusCode] = 429
	attributes["http.status_text"] = errorMsg

	span := constructExceptionServerSpan(attributes, ptrace.StatusCodeError)
	filtered, _ := makeHTTP(span)

	res := pcommon.NewResource()
	isError, isFault, isThrottle, filtered, cause := makeCause(span, filtered, res)

	assert.True(t, isError)
	assert.False(t, isFault)
	assert.True(t, isThrottle)
	assert.NotNil(t, filtered)
	assert.NotNil(t, cause)
}

func constructExceptionServerSpan(attributes map[string]interface{}, statuscode ptrace.StatusCode) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/widgets")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewSpanStatus()
	status.SetCode(statuscode)
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func TestParseExceptionWithoutStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionWithoutMessage(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := ""
	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Empty(t, exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionWithJavaStacktraceNoCause(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
}

func TestParseExceptionWithStacktraceNotJava(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)`

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Empty(t, exceptions[0].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithoutStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Empty(t, exceptions[1].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithoutMessageOrStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Empty(t, *exceptions[1].Message)
	assert.Empty(t, exceptions[1].Stack)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithStacktrace(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
Caused by: java.lang.IllegalArgumentException: bad argument
	at org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute(ThrowableCollector.java:73)
	at org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively(NodeTestTask.java)`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "ThrowableCollector.java", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 73, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "NodeTestTask.java", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithJavaStacktraceAndCauseWithStacktraceSkipCommonAndSuppressedAndMalformed(t *testing.T) {
	exceptionType := "com.foo.Exception"
	message := "Error happened"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `java.lang.IllegalStateException: state is not legal
	at io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException(RecordEventsReadableSpanTest.java:626)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke0(Native Method)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62)afaefaef
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke
	at java.base/jdk.internal.reflect.NativeMethodAccessorImpl.invoke(NativeMethodAccessorImpl.java:62
	at java.base/java.util.ArrayList.forEach(ArrayList.java:)
	Suppressed: Resource$CloseFailException: Resource ID = 2
		at Resource.close(Resource.java:26)	
		at Foo3.main(Foo3.java:5)
	Suppressed: Resource$CloseFailException: Resource ID = 1
		at Resource.close(Resource.java:26)
		at Foo3.main(Foo3.java:5)
Caused by: java.lang.IllegalArgumentException: bad argument
	at org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute(ThrowableCollector.java:73)
	at org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively(NodeTestTask.java)
	... 99 more`

	exceptions := parseException(exceptionType, message, stacktrace, "java")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "com.foo.Exception", *exceptions[0].Type)
	assert.Equal(t, "Error happened", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 4)
	assert.Equal(t, "io.opentelemetry.sdk.trace.RecordEventsReadableSpanTest.recordException", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "RecordEventsReadableSpanTest.java", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 626, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke0", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "Native Method", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "jdk.internal.reflect.NativeMethodAccessorImpl.invoke", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "NativeMethodAccessorImpl.java", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 62, *exceptions[0].Stack[2].Line)
	// This is technically malformed but close enough to our format we may as well accept it.
	assert.Equal(t, "java.util.ArrayList.forEach", *exceptions[0].Stack[3].Label)
	assert.Equal(t, "ArrayList.java", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[3].Line)
	assert.NotNil(t, exceptions[1].ID)
	assert.Equal(t, exceptions[1].ID, exceptions[0].Cause)
	assert.Equal(t, "java.lang.IllegalArgumentException", *exceptions[1].Type)
	assert.Equal(t, "bad argument", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.ThrowableCollector.execute", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "ThrowableCollector.java", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 73, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "org.junit.platform.engine.support.hierarchical.NodeTestTask.executeRecursively", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "NodeTestTask.java", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceNoCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError: bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)

	assert.NotEmpty(t, exceptions[1].ID)
	assert.Equal(t, "ValueError", *exceptions[1].Type)
	assert.Equal(t, "bad value", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "greet", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "foo.py", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 5, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "greet_many", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "bar.py", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 10, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndMultiLineCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError: bad value
with more on
new lines

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)

	assert.NotEmpty(t, exceptions[1].ID)
	assert.Equal(t, "ValueError", *exceptions[1].Type)
	assert.Equal(t, "bad value\nwith more on\nnew lines", *exceptions[1].Message)
	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, "greet", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "foo.py", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 5, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "greet_many", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "bar.py", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 10, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceMalformedLines(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "main.py", line 14 in <module>
    greet_many(['Chad', 'Dan', 1])
  File "main.py", lin 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "main.py", line 14, fin <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[2].Line)
}

func TestParseExceptionWithPythonStacktraceAndMalformedCause(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
ValueError: bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionWithPythonStacktraceAndMalformedCauseMessage(t *testing.T) {
	exceptionType := "TypeError"
	message := "must be str, not int"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `Traceback (most recent call last):
  File "bar.py", line 10, in greet_many
    greet(person)
  File "foo.py", line 5, in greet
    print(greeting + ', ' + who_to_greet(someone))
ValueError bad value

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "main.py", line 14, in <module>
    greet_many(['Chad', 'Dan', 1])
  File "greetings.py", line 12, in greet_many
    print('hi, ' + person)
TypeError: must be str, not int`

	exceptions := parseException(exceptionType, message, stacktrace, "python")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "must be str, not int", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "greet_many", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "greetings.py", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "<module>", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "main.py", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 14, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionWithJavaScriptStacktrace(t *testing.T) {
	exceptionType := "TypeError"
	message := "Cannot read property 'value' of null"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `TypeError: Cannot read property 'value' of null
    at speedy (/home/gbusey/file.js:6:11)
    at makeFaster (/home/gbusey/file.js:5:3)
    at Object.<anonymous> (/home/gbusey/file.js:10:1)
    at node.js:906:3
    at Array.forEach (native)
    at native`

	exceptions := parseException(exceptionType, message, stacktrace, "javascript")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "Cannot read property 'value' of null", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 6)
	assert.Equal(t, "speedy ", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "/home/gbusey/file.js", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 6, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "makeFaster ", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "/home/gbusey/file.js", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 5, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "Object.<anonymous> ", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "/home/gbusey/file.js", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 10, *exceptions[0].Stack[2].Line)
	assert.Equal(t, "", *exceptions[0].Stack[3].Label)
	assert.Equal(t, "node.js", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 906, *exceptions[0].Stack[3].Line)
	assert.Equal(t, "Array.forEach ", *exceptions[0].Stack[4].Label)
	assert.Equal(t, "native", *exceptions[0].Stack[4].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[4].Line)
	assert.Equal(t, "", *exceptions[0].Stack[5].Label)
	assert.Equal(t, "native", *exceptions[0].Stack[5].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[5].Line)
}

func TestParseExceptionWithStacktraceNotJavaScript(t *testing.T) {
	exceptionType := "TypeError"
	message := "Cannot read property 'value' of null"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `TypeError: Cannot read property 'value' of null
    at speedy (/home/gbusey/file.js:6:11)
    at makeFaster (/home/gbusey/file.js:5:3)
    at Object.<anonymous> (/home/gbusey/file.js:10:1)`

	exceptions := parseException(exceptionType, message, stacktrace, "")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "Cannot read property 'value' of null", *exceptions[0].Message)
	assert.Empty(t, exceptions[0].Stack)
}

func TestParseExceptionWithJavaScriptStactracekMalformedLines(t *testing.T) {
	exceptionType := "TypeError"
	message := "Cannot read property 'value' of null"
	// We ignore the exception type / message from the stacktrace
	stacktrace := `TypeError: Cannot read property 'value' of null
    at speedy (/home/gbusey/file.js)
    at makeFaster (/home/gbusey/file.js:5:3)malformed123
    at Object.<anonymous> (/home/gbusey/file.js:10`

	exceptions := parseException(exceptionType, message, stacktrace, "javascript")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "TypeError", *exceptions[0].Type)
	assert.Equal(t, "Cannot read property 'value' of null", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 1)
	assert.Equal(t, "speedy ", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "/home/gbusey/file.js", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[0].Line)
}

func TestParseExceptionWithSimpleStacktrace(t *testing.T) {
	exceptionType := "System.FormatException"
	message := "Input string was not in a correct format"

	// We ignore the exception type / message from the stacktrace
	stacktrace := `System.FormatException: Input string was not in a correct format.
	at System.Number.ThrowOverflowOrFormatException(ParsingStatus status, TypeCode type)
	at System.Number.ParseInt32(ReadOnlySpan1 value, NumberStyles styles, NumberFormatInfo info)
	at System.Int32.Parse(String s)
	at MyNamespace.IntParser.Parse(String s) in C:\apps\MyNamespace\IntParser.cs:line 11
	at MyNamespace.Program.Main(String[] args) in C:\apps\MyNamespace\Program.cs:line 12`

	exceptions := parseException(exceptionType, message, stacktrace, "dotnet")
	assert.Len(t, exceptions, 1)
	assert.Equal(t, "System.FormatException", *exceptions[0].Type)
	assert.Equal(t, "Input string was not in a correct format", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 5)
	assert.Equal(t, "System.Number.ThrowOverflowOrFormatException(ParsingStatus status, TypeCode type)", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "System.Number.ParseInt32(ReadOnlySpan1 value, NumberStyles styles, NumberFormatInfo info)", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "System.Int32.Parse(String s)", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[2].Line)
	assert.Equal(t, "MyNamespace.IntParser.Parse(String s)", *exceptions[0].Stack[3].Label)
	assert.Equal(t, "C:\\apps\\MyNamespace\\IntParser.cs", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 11, *exceptions[0].Stack[3].Line)
	assert.Equal(t, "MyNamespace.Program.Main(String[] args)", *exceptions[0].Stack[4].Label)
	assert.Equal(t, "C:\\apps\\MyNamespace\\Program.cs", *exceptions[0].Stack[4].Path)
	assert.Equal(t, 12, *exceptions[0].Stack[4].Line)
}

func TestParseExceptionWithInnerExceptionStacktrace(t *testing.T) {
	exceptionType := "System.Exception"
	message := "test"

	// We ignore the exception type / message from the stacktrace
	stacktrace := `System.Exception: test
	at integration_test_app.Controllers.AppController.OutgoingHttp() in /Users/bhautip/Documents/otel-dotnet/aws-otel-dotnet/integration-test-app/integration-test-app/Controllers/AppController.cs:line 21
	at lambda_method(Closure , Object , Object[] )
	at Microsoft.Extensions.Internal.ObjectMethodExecutor.Execute(Object target, Object[] parameters)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ActionMethodExecutor.SyncObjectResultExecutor.Execute(IActionResultTypeMapper mapper, ObjectMethodExecutor executor, Object controller, Object[] arguments)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ControllerActionInvoker.<InvokeActionMethodAsync>g__Logged|12_1(ControllerActionInvoker invoker)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ControllerActionInvoker.<InvokeNextActionFilterAsync>g__Awaited|10_0(ControllerActionInvoker invoker, Task lastTask, State next, Scope scope, Object state, Boolean isCompleted)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ControllerActionInvoker.Rethrow(ActionExecutedContextSealed context)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ControllerActionInvoker.Next(State& next, Scope& scope, Object& state, Boolean& isCompleted)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ControllerActionInvoker.InvokeInnerFilterAsync()
	--- End of stack trace from previous location where exception was thrown ---
	at Microsoft.AspNetCore.Mvc.Infrastructure.ResourceInvoker.<InvokeFilterPipelineAsync>g__Awaited|19_0(ResourceInvoker invoker, Task lastTask, State next, Scope scope, Object state, Boolean isCompleted)
	at Microsoft.AspNetCore.Mvc.Infrastructure.ResourceInvoker.<InvokeAsync>g__Logged|17_1(ResourceInvoker invoker)
	at Microsoft.AspNetCore.Routing.EndpointMiddleware.<Invoke>g__AwaitRequestTask|6_0(Endpoint endpoint, Task requestTask, ILogger logger)
	at Microsoft.AspNetCore.Authorization.AuthorizationMiddleware.Invoke(HttpContext context)
	at Microsoft.AspNetCore.Diagnostics.DeveloperExceptionPageMiddleware.Invoke(HttpContext context)`

	exceptions := parseException(exceptionType, message, stacktrace, "dotnet")
	assert.Len(t, exceptions, 1)
	assert.Equal(t, "System.Exception", *exceptions[0].Type)
	assert.Equal(t, "test", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 14)
	assert.Equal(t, "integration_test_app.Controllers.AppController.OutgoingHttp()", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "/Users/bhautip/Documents/otel-dotnet/aws-otel-dotnet/integration-test-app/integration-test-app/Controllers/AppController.cs", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 21, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "Microsoft.AspNetCore.Mvc.Infrastructure.ResourceInvoker.<InvokeFilterPipelineAsync>g__Awaited|19_0(ResourceInvoker invoker, Task lastTask, State next, Scope scope, Object state, Boolean isCompleted)", *exceptions[0].Stack[9].Label)
	assert.Equal(t, "", *exceptions[0].Stack[9].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[9].Line)
}

func TestParseExceptionWithMalformedStacktrace(t *testing.T) {
	exceptionType := "System.Exception"
	message := "test"

	// We ignore the exception type / message from the stacktrace
	stacktrace := `System.Exception: test
	at integration_test_app.Controllers.AppController.OutgoingHttp() in /Users/bhautip/Documents/otel-dotnet/aws-otel-dotnet/integration-test-app/integration-test-app/Controllers/AppController.cs:line 21
	at Microsoft.AspNetCore.Diagnostics.DeveloperExceptionPageMiddleware.Invoke(HttpContext context malformed
	at System.Net.Http.HttpConnectionPool.ConnectAsync(HttpRequestMessage request, Boolean allowHttp2, CancellationToken cancellationToken) non-malformed`

	exceptions := parseException(exceptionType, message, stacktrace, "dotnet")
	assert.Len(t, exceptions, 1)
	assert.Equal(t, "System.Exception", *exceptions[0].Type)
	assert.Equal(t, "test", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "integration_test_app.Controllers.AppController.OutgoingHttp()", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "/Users/bhautip/Documents/otel-dotnet/aws-otel-dotnet/integration-test-app/integration-test-app/Controllers/AppController.cs", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 21, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "System.Net.Http.HttpConnectionPool.ConnectAsync(HttpRequestMessage request, Boolean allowHttp2, CancellationToken cancellationToken)", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 0, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionPhpStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from grandparent"

	stacktrace := `Exception: Thrown from grandparent
	at grandparent_func(test.php:56)
	at parent_func(test.php:51)
	at child_func(test.php:44)
	at main(test.php:63)`

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from grandparent", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 4)
	assert.Equal(t, "grandparent_func", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 56, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "parent_func", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 51, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "child_func", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 44, *exceptions[0].Stack[2].Line)
	assert.Equal(t, "main", *exceptions[0].Stack[3].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 63, *exceptions[0].Stack[3].Line)
}

func TestParseExceptionPhpWithoutStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from grandparent"

	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from grandparent", *exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionPhpStacktraceWithCause(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from class B"

	stacktrace := `Exception: Thrown from class B
	at B.exc(test.php:59)
	at fail(test.php:81)
	at main(test.php:89)
Caused by: Exception: Thrown from class A`

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 2)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from class B", *exceptions[0].Message)
	assert.Equal(t, *exceptions[0].Cause, *exceptions[1].ID)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "B.exc", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 59, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "fail", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 81, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "main", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 89, *exceptions[0].Stack[2].Line)

	assert.Equal(t, "Exception", *exceptions[1].Type)
	assert.Equal(t, "Thrown from class A", *exceptions[1].Message)
	assert.Empty(t, exceptions[1].Stack)
}

func TestParseExceptionPhpStacktraceWithCauseAndStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from class B"

	stacktrace := `Exception: Thrown from class B
	at B.exc(test.php:59)
	at fail(test.php:81)
	at main(test.php:89)
Caused by: Exception: Thrown from class A
	at A.exc(test.php:48)
	at B.exc(test.php:56)
	... 2 more`

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 2)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from class B", *exceptions[0].Message)
	assert.Equal(t, *exceptions[0].Cause, *exceptions[1].ID)
	assert.Len(t, exceptions[0].Stack, 3)
	assert.Equal(t, "B.exc", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 59, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "fail", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 81, *exceptions[0].Stack[1].Line)
	assert.Equal(t, "main", *exceptions[0].Stack[2].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[2].Path)
	assert.Equal(t, 89, *exceptions[0].Stack[2].Line)

	assert.Len(t, exceptions[1].Stack, 2)
	assert.NotEmpty(t, exceptions[1].ID)
	assert.Equal(t, "Exception", *exceptions[1].Type)
	assert.Equal(t, "Thrown from class A", *exceptions[1].Message)
	assert.Equal(t, "A.exc", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 48, *exceptions[1].Stack[0].Line)
	assert.Equal(t, "B.exc", *exceptions[1].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[1].Stack[1].Path)
	assert.Equal(t, 56, *exceptions[1].Stack[1].Line)
}

func TestParseExceptionPhpStacktraceWithMultipleCause(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from class C"

	stacktrace := `Exception: Thrown from class C
	at C.exc(test.php:74)
	at main(test.php:89)
Caused by: Exception: Thrown from class B
	at B.exc(test.php:59)
	at C.exc(test.php:71)
	... 3 more
Caused by: Exception: Thrown from class A
	at A.exc(test.php:48)
	at B.exc(test.php:56)
	... 4 more`

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 3)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from class C", *exceptions[0].Message)
	assert.Equal(t, *exceptions[0].Cause, *exceptions[1].ID)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "C.exc", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 74, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "main", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 89, *exceptions[0].Stack[1].Line)

	assert.Len(t, exceptions[1].Stack, 2)
	assert.Equal(t, *exceptions[1].Cause, *exceptions[2].ID)
	assert.Equal(t, "Exception", *exceptions[1].Type)
	assert.Equal(t, "Thrown from class B", *exceptions[1].Message)
	assert.Equal(t, "B.exc", *exceptions[1].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[1].Stack[0].Path)
	assert.Equal(t, 59, *exceptions[1].Stack[0].Line)

	assert.Len(t, exceptions[2].Stack, 2)
	assert.NotEmpty(t, exceptions[2].ID)
	assert.Equal(t, "Exception", *exceptions[2].Type)
	assert.Equal(t, "Thrown from class A", *exceptions[2].Message)
	assert.Equal(t, "B.exc", *exceptions[2].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[2].Stack[1].Path)
	assert.Equal(t, 56, *exceptions[2].Stack[1].Line)
}

func TestParseExceptionPhpStacktraceMalformedLines(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from class B"

	stacktrace := `Exception: Thrown from class B
	at B.exc(test.php:59)
	at fail(test.php:81 malformed
	at main(test.php:89)`

	exceptions := parseException(exceptionType, message, stacktrace, "php")

	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from class B", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 2)
	assert.Equal(t, "B.exc", *exceptions[0].Stack[0].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 59, *exceptions[0].Stack[0].Line)
	assert.Equal(t, "main", *exceptions[0].Stack[1].Label)
	assert.Equal(t, "test.php", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 89, *exceptions[0].Stack[1].Line)
}

func TestParseExceptionGoWithoutStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "Thrown from grandparent"

	stacktrace := ""

	exceptions := parseException(exceptionType, message, stacktrace, "go")

	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "Thrown from grandparent", *exceptions[0].Message)
	assert.Nil(t, exceptions[0].Stack)
}

func TestParseExceptionGoWithStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "error message"

	stacktrace := `goroutine 19 [running]:
go.opentelemetry.io/otel/sdk/trace.recordStackTrace(0x0, 0x0)
	otel-go-core/opentelemetry-go/sdk/trace/span.go:323 +0x9b
go.opentelemetry.io/otel/sdk/trace.(*span).RecordError(0xc0003a6000, 0x14a5f00, 0xc00038c000, 0xc000390140, 0x3, 0x4)
	otel-go-core/opentelemetry-go/sdk/trace/span.go:302 +0x3fc
go.opentelemetry.io/otel/sdk/trace.TestRecordErrorWithStackTrace(0xc000102900)
	otel-go-core/opentelemetry-go/sdk/trace/trace_test.go:1167 +0x3ef
testing.tRunner(0xc000102900, 0x1484410)
	/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go:1193 +0x1a3
created by testing.(*T).Run
	/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go:1238 +0x63c`

	exceptions := parseException(exceptionType, message, stacktrace, "go")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "error message", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 5)

	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[0].Label, "go.opentelemetry.io/otel/sdk/trace.recordStackTrace"))
	assert.Equal(t, "otel-go-core/opentelemetry-go/sdk/trace/span.go", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 323, *exceptions[0].Stack[0].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[1].Label, "go.opentelemetry.io/otel/sdk/trace.(*span).RecordError"))
	assert.Equal(t, "otel-go-core/opentelemetry-go/sdk/trace/span.go", *exceptions[0].Stack[1].Path)
	assert.Equal(t, 302, *exceptions[0].Stack[1].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[3].Label, "testing.tRunner"))
	assert.Equal(t, "/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go", *exceptions[0].Stack[3].Path)
	assert.Equal(t, 1193, *exceptions[0].Stack[3].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[4].Label, "created by testing.(*T).Run"))
	assert.Equal(t, "/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go", *exceptions[0].Stack[4].Path)
	assert.Equal(t, 1238, *exceptions[0].Stack[4].Line)
}

func TestParseMultipleExceptionGoWithStacktrace(t *testing.T) {
	exceptionType := "Exception"
	message := "panic"

	stacktrace := `goroutine 19 [running]:
go.opentelemetry.io/otel/sdk/trace.recordStackTrace(0x0, 0x0)
	Documents/otel-go-core/opentelemetry-go/sdk/trace/span.go:318 +0x9b
go.opentelemetry.io/otel/sdk/trace.(*span).End(0xc000082300, 0xc0000a0040, 0x1, 0x1)
	Documents/otel-go-core/opentelemetry-go/sdk/trace/span.go:252 +0x4ee
panic(0x1414f00, 0xc0000a0050)
	/usr/local/Cellar/go/1.16.3/libexec/src/runtime/panic.go:971 +0x4c7
go.opentelemetry.io/otel/sdk/trace.TestSpanCapturesPanicWithStackTrace.func1()
	Documents/otel-go-core/opentelemetry-go/sdk/trace/trace_test.go:1425 +0x225
github.com/stretchr/testify/assert.didPanic.func1(0xc0001ad0e8, 0xc0001ad0d7, 0xc0001ad0d8, 0xc00009e048)
	go/pkg/mod/github.com/stretchr/testify@v1.7.0/assert/assertions.go:1018 +0xb8
github.com/stretchr/testify/assert.didPanic(0xc00009e048, 0x14a5b00, 0x0, 0x0, 0x0, 0x0)
	go/pkg/mod/github.com/stretchr/testify@v1.7.0/assert/assertions.go:1020 +0x85
github.com/stretchr/testify/assert.PanicsWithError(0x14a5b60, 0xc000186600, 0x146e31c, 0xd, 0xc00009e048, 0x0, 0x0, 0x0, 0xc000038900)
	go/pkg/mod/github.com/stretchr/testify@v1.7.0/assert/assertions.go:1071 +0x10c
goroutine 26 [running]:
github.com/stretchr/testify/require.PanicsWithError(0x14a7328, 0xc000186600, 0x146e31c, 0xd, 0xc00009e048, 0x0, 0x0, 0x0)
	go/pkg/mod/github.com/stretchr/testify@v1.7.0/require/require.go:1607 +0x15e
go.opentelemetry.io/otel/sdk/trace.TestSpanCapturesPanicWithStackTrace(0xc000186600)
	Documents/otel-go-core/opentelemetry-go/sdk/trace/trace_test.go:1427 +0x33a
testing.tRunner(0xc000186600, 0x1484440)
	/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go:1193 +0x1a3
created by testing.(*T).Run
	/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go:1238 +0x63c`

	exceptions := parseException(exceptionType, message, stacktrace, "go")
	assert.Len(t, exceptions, 1)
	assert.NotEmpty(t, exceptions[0].ID)
	assert.Equal(t, "Exception", *exceptions[0].Type)
	assert.Equal(t, "panic", *exceptions[0].Message)
	assert.Len(t, exceptions[0].Stack, 11)

	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[0].Label, "go.opentelemetry.io/otel/sdk/trace.recordStackTrace"))
	assert.Equal(t, "Documents/otel-go-core/opentelemetry-go/sdk/trace/span.go", *exceptions[0].Stack[0].Path)
	assert.Equal(t, 318, *exceptions[0].Stack[0].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[7].Label, "github.com/stretchr/testify/require.PanicsWithError"))
	assert.Equal(t, "go/pkg/mod/github.com/stretchr/testify@v1.7.0/require/require.go", *exceptions[0].Stack[7].Path)
	assert.Equal(t, 1607, *exceptions[0].Stack[7].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[8].Label, "go.opentelemetry.io/otel/sdk/trace.TestSpanCapturesPanicWithStackTrace"))
	assert.Equal(t, "Documents/otel-go-core/opentelemetry-go/sdk/trace/trace_test.go", *exceptions[0].Stack[8].Path)
	assert.Equal(t, 1427, *exceptions[0].Stack[8].Line)
	assert.True(t, strings.HasPrefix(*exceptions[0].Stack[10].Label, "created by testing.(*T).Run"))
	assert.Equal(t, "/usr/local/Cellar/go/1.16.3/libexec/src/testing/testing.go", *exceptions[0].Stack[10].Path)
	assert.Equal(t, 1238, *exceptions[0].Stack[10].Line)
}
