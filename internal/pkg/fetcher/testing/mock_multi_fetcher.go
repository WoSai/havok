// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/wosai/havok/pkg/plugin (interfaces: Fetcher)

// Package mock_plugin is a generated GoMock package.
package testing

import (
	"context"
	"reflect"

	"github.com/golang/mock/gomock"
	"github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
)

// MockFetcher is a mock of Fetcher interface.
type MockFetcher struct {
	ctrl     *gomock.Controller
	recorder *MockFetcherMockRecorder
}

// MockFetcherMockRecorder is the mock recorder for MockFetcher.
type MockFetcherMockRecorder struct {
	mock *MockFetcher
}

// NewMockFetcher creates a new mock instance.
func NewMockFetcher(ctrl *gomock.Controller) *MockFetcher {
	mock := &MockFetcher{ctrl: ctrl}
	mock.recorder = &MockFetcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFetcher) EXPECT() *MockFetcherMockRecorder {
	return m.recorder
}

// Apply mocks base method.
func (m *MockFetcher) Apply(arg0 interface{}) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Apply", arg0)
}

// Apply indicates an expected call of Apply.
func (mr *MockFetcherMockRecorder) Apply(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockFetcher)(nil).Apply), arg0)
}

// Fetch mocks base method.
func (m *MockFetcher) Fetch(arg0 context.Context, arg1 chan<- *genproto.LogRecord) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Fetch", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Fetch indicates an expected call of Fetch.
func (mr *MockFetcherMockRecorder) Fetch(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Fetch", reflect.TypeOf((*MockFetcher)(nil).Fetch), arg0, arg1)
}

// Name mocks base method.
func (m *MockFetcher) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFetcherMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFetcher)(nil).Name))
}

// WithDecoder mocks base method.
func (m *MockFetcher) WithDecoder(arg0 plugin.LogDecoder) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "WithDecoder", arg0)
}

// WithDecoder indicates an expected call of WithDecoder.
func (mr *MockFetcherMockRecorder) WithDecoder(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithDecoder", reflect.TypeOf((*MockFetcher)(nil).WithDecoder), arg0)
}