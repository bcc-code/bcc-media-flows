// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/bcc-code/bcc-media-flows/services/vidispine (interfaces: VSClient)

// Package vsmock is a generated GoMock package.
package vsmock

import (
	reflect "reflect"

	vsapi "github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	gomock "go.uber.org/mock/gomock"
)

// MockVSClient is a mock of VSClient interface.
type MockVSClient struct {
	ctrl     *gomock.Controller
	recorder *MockVSClientMockRecorder
}

func (m *MockVSClient) AddToItemMetadataField(itemID, key, value string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockVSClient) CreatePlaceholder(ingestType vsapi.PlaceholderType, title string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockVSClient) CreateThumbnails(assetID string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockVSClient) GetJob(jobID string) (*vsapi.JobDocument, error) {
	//TODO implement me
	panic("implement me")
}

// MockVSClientMockRecorder is the mock recorder for MockVSClient.
type MockVSClientMockRecorder struct {
	mock *MockVSClient
}

// NewMockVSClient creates a new mock instance.
func NewMockVSClient(ctrl *gomock.Controller) *MockVSClient {
	mock := &MockVSClient{ctrl: ctrl}
	mock.recorder = &MockVSClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVSClient) EXPECT() *MockVSClientMockRecorder {
	return m.recorder
}

// AddShapeToItem mocks base method.
func (m *MockVSClient) AddShapeToItem(arg0, arg1, arg2 string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddShapeToItem", arg0, arg1, arg2)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddShapeToItem indicates an expected call of AddShapeToItem.
func (mr *MockVSClientMockRecorder) AddShapeToItem(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddShapeToItem", reflect.TypeOf((*MockVSClient)(nil).AddShapeToItem), arg0, arg1, arg2)
}

// AddSidecarToItem mocks base method.
func (m *MockVSClient) AddSidecarToItem(arg0, arg1, arg2 string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddSidecarToItem", arg0, arg1, arg2)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddSidecarToItem indicates an expected call of AddSidecarToItem.
func (mr *MockVSClientMockRecorder) AddSidecarToItem(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddSidecarToItem", reflect.TypeOf((*MockVSClient)(nil).AddSidecarToItem), arg0, arg1, arg2)
}

// GetChapterMeta mocks base method.
func (m *MockVSClient) GetChapterMeta(arg0 string, arg1, arg2 float64) (map[string]*vsapi.MetadataResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChapterMeta", arg0, arg1, arg2)
	ret0, _ := ret[0].(map[string]*vsapi.MetadataResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetChapterMeta indicates an expected call of GetChapterMeta.
func (mr *MockVSClientMockRecorder) GetChapterMeta(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChapterMeta", reflect.TypeOf((*MockVSClient)(nil).GetChapterMeta), arg0, arg1, arg2)
}

// GetMetadata mocks base method.
func (m *MockVSClient) GetMetadata(arg0 string) (*vsapi.MetadataResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMetadata", arg0)
	ret0, _ := ret[0].(*vsapi.MetadataResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMetadata indicates an expected call of GetMetadata.
func (mr *MockVSClientMockRecorder) GetMetadata(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMetadata", reflect.TypeOf((*MockVSClient)(nil).GetMetadata), arg0)
}

// GetSequence mocks base method.
func (m *MockVSClient) GetSequence(arg0 string) (*vsapi.SequenceDocument, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSequence", arg0)
	ret0, _ := ret[0].(*vsapi.SequenceDocument)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSequence indicates an expected call of GetSequence.
func (mr *MockVSClientMockRecorder) GetSequence(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSequence", reflect.TypeOf((*MockVSClient)(nil).GetSequence), arg0)
}

// GetShapes mocks base method.
func (m *MockVSClient) GetShapes(arg0 string) (*vsapi.ShapeResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetShapes", arg0)
	ret0, _ := ret[0].(*vsapi.ShapeResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetShapes indicates an expected call of GetShapes.
func (mr *MockVSClientMockRecorder) GetShapes(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetShapes", reflect.TypeOf((*MockVSClient)(nil).GetShapes), arg0)
}

// RegisterFile mocks base method.
func (m *MockVSClient) RegisterFile(arg0 string, arg1 vsapi.FileState) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterFile", arg0, arg1)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterFile indicates an expected call of RegisterFile.
func (mr *MockVSClientMockRecorder) RegisterFile(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterFile", reflect.TypeOf((*MockVSClient)(nil).RegisterFile), arg0, arg1)
}

// SetItemMetadataField mocks base method.
func (m *MockVSClient) SetItemMetadataField(arg0, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetItemMetadataField", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetItemMetadataField indicates an expected call of SetItemMetadataField.
func (mr *MockVSClientMockRecorder) SetItemMetadataField(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetItemMetadataField", reflect.TypeOf((*MockVSClient)(nil).SetItemMetadataField), arg0, arg1, arg2)
}
