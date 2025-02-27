// Copyright 2019-present Open Networking Foundation.
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

package gnmi

import (
	"fmt"
	"github.com/golang/mock/gomock"
	td1 "github.com/onosproject/config-models/modelplugin/testdevice-1.0.0/testdevice_1_0_0"
	changetypes "github.com/onosproject/onos-api/go/onos/config/change"
	devicechange "github.com/onosproject/onos-api/go/onos/config/change/device"
	networkchange "github.com/onosproject/onos-api/go/onos/config/change/network"
	devicetype "github.com/onosproject/onos-api/go/onos/config/device"
	configmodel "github.com/onosproject/onos-config-model/pkg/model"
	topodevice "github.com/onosproject/onos-config/pkg/device"
	"github.com/onosproject/onos-config/pkg/dispatcher"
	"github.com/onosproject/onos-config/pkg/manager"
	"github.com/onosproject/onos-config/pkg/modelregistry"
	networkchangestore "github.com/onosproject/onos-config/pkg/store/change/network"
	"github.com/onosproject/onos-config/pkg/store/device/cache"
	"github.com/onosproject/onos-config/pkg/store/stream"
	mockstore "github.com/onosproject/onos-config/pkg/test/mocks/store"
	mockcache "github.com/onosproject/onos-config/pkg/test/mocks/store/cache"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/ygot"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"sync"
	"testing"
	"time"
)

// AllMocks is a place to hold references to mock stores and caches. It can't be put into test/mocks/store
// because of circular dependencies.
type AllMocks struct {
	MockStores      *mockstore.MockStores
	MockDeviceCache *mockcache.MockCache
}

// TestMain should only contain static data.
// It is run once for all tests - each test is then run on its own thread, so if
// anything is shared the order of it's modification is not deterministic
// Also there can only be one TestMain per package
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func setUpWatchMock(mocks *AllMocks) {
	now := time.Now()
	watchChange := networkchange.NetworkChange{
		ID:       "",
		Index:    0,
		Revision: 0,
		Created:  now,
		Updated:  now,
		Changes:  nil,
		Refs:     nil,
		Deleted:  false,
	}
	watchUpdate := networkchange.NetworkChange{
		ID:       "",
		Index:    0,
		Revision: 0,
		Created:  now,
		Updated:  now,
		Changes:  nil,
		Refs: []*networkchange.DeviceChangeRef{
			{
				DeviceChangeID: "network-change-1:device-change-1",
			},
		},
		Deleted: false,
	}

	config1Value01, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2a", devicechange.NewTypedValueUint(11, 8), false)
	config1Value02, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2g", devicechange.NewTypedValueBool(true), true)

	deviceChange := devicechange.DeviceChange{
		ID:       "",
		Index:    0,
		Revision: 0,
		Status: changetypes.Status{
			Phase:   0,
			State:   changetypes.State_COMPLETE,
			Reason:  0,
			Message: "",
		},
		Created: now,
		Updated: now,
		NetworkChange: devicechange.NetworkChangeRef{
			ID:    "",
			Index: 0,
		},
		Change: &devicechange.Change{
			DeviceID:      "",
			DeviceVersion: "",
			Values: []*devicechange.ChangeValue{
				config1Value01, config1Value02,
			},
		},
	}

	mocks.MockStores.NetworkChangesStore.EXPECT().Watch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(c chan<- stream.Event, opts ...networkchangestore.WatchOption) (stream.Context, error) {
			go func() {
				c <- stream.Event{Object: &watchChange}
				c <- stream.Event{Object: &watchUpdate}
				close(c)
			}()
			return stream.NewContext(func() {

			}), nil
		}).AnyTimes()

	mocks.MockStores.DeviceChangesStore.EXPECT().Watch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(deviceId devicetype.VersionedID, c chan<- stream.Event, opts ...networkchangestore.WatchOption) (stream.Context, error) {
			go func() {
				c <- stream.Event{Object: &deviceChange}
				close(c)
			}()
			return stream.NewContext(func() {

			}), nil
		}).AnyTimes()
}

func setUpListMock(mocks *AllMocks) {
	mocks.MockStores.DeviceChangesStore.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(device devicetype.VersionedID, c chan<- *devicechange.DeviceChange) (stream.Context, error) {
			go func() {
				close(c)
			}()
			return stream.NewContext(func() {

			}), nil
		}).AnyTimes()
}

func setUpChangesMock(mocks *AllMocks) {
	configValue01, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2a", devicechange.NewTypedValueUint(13, 8), false)
	configValue02, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2b", devicechange.NewTypedValueDecimal(14567, 4), false)
	configValue03, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2d", devicechange.NewTypedValueDecimal(11, 1), false)
	configValue04, _ := devicechange.NewChangeValue("/cont1a/list2a[name=first]/name", devicechange.NewTypedValueString("first"), false)
	configValue05, _ := devicechange.NewChangeValue("/cont1a/list2a[name=first]/tx-power", devicechange.NewTypedValueUint(19, 16), false)
	configValue06, _ := devicechange.NewChangeValue("/cont1a/list2a[name=first]/ref2d", devicechange.NewTypedValueDecimal(11, 1), false)
	configValue07, _ := devicechange.NewChangeValue("/cont1a/list4[id=first]/id", devicechange.NewTypedValueString("first"), false)
	configValue08, _ := devicechange.NewChangeValue("/cont1a/list4[id=first]/leaf4b", devicechange.NewTypedValueString("initial value"), false)
	configValue09, _ := devicechange.NewChangeValue("/cont1a/list5[key1=abc][key2=8]/key1", devicechange.NewTypedValueString("abc"), false)
	configValue10, _ := devicechange.NewChangeValue("/cont1a/list5[key1=abc][key2=8]/key2", devicechange.NewTypedValueUint(8, 16), false)
	configValue11, _ := devicechange.NewChangeValue("/cont1a/list5[key1=abc][key2=8]/leaf5a", devicechange.NewTypedValueString("Leaf 5a"), false)
	configValue12, _ := devicechange.NewChangeValue("/cont1a/list4[id=first]/list4a[fkey1=abc][fkey2=8]/fkey1", devicechange.NewTypedValueString("abc"), false)
	configValue13, _ := devicechange.NewChangeValue("/cont1a/list4[id=first]/list4a[fkey1=abc][fkey2=8]/fkey2", devicechange.NewTypedValueUint(8, 16), false)
	configValue14, _ := devicechange.NewChangeValue("/cont1a/list4[id=first]/list4a[fkey1=abc][fkey2=8]/displayname", devicechange.NewTypedValueString("this is a list"), false)
	configValue15, _ := devicechange.NewChangeValue("/cont1a/leaf1a", devicechange.NewTypedValueString("test val"), false)

	change1 := devicechange.Change{
		Values: []*devicechange.ChangeValue{
			configValue01,
			configValue02,
			configValue03,
			configValue04,
			configValue05,
			configValue06,
			configValue07,
			configValue08,
			configValue09,
			configValue10,
			configValue11,
			configValue12,
			configValue13,
			configValue14,
			configValue15,
		},
		DeviceID:      devicetype.ID("Device1"),
		DeviceVersion: "1.0.0",
	}
	deviceChange1 := &devicechange.DeviceChange{
		Change: &change1,
		ID:     "Change",
		Status: changetypes.Status{
			Phase:   changetypes.Phase_CHANGE,
			State:   changetypes.State_COMPLETE,
			Reason:  0,
			Message: "",
		},
	}
	mocks.MockStores.DeviceStateStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*devicechange.PathValue{
		{Path: configValue01.Path, Value: configValue01.Value},
		{Path: configValue02.Path, Value: configValue02.Value},
		{Path: configValue03.Path, Value: configValue03.Value},
		{Path: configValue04.Path, Value: configValue04.Value},
		{Path: configValue05.Path, Value: configValue05.Value},
		{Path: configValue06.Path, Value: configValue06.Value},
		{Path: configValue07.Path, Value: configValue07.Value},
		{Path: configValue08.Path, Value: configValue08.Value},
		{Path: configValue09.Path, Value: configValue09.Value},
		{Path: configValue10.Path, Value: configValue10.Value},
		{Path: configValue11.Path, Value: configValue11.Value},
		{Path: configValue12.Path, Value: configValue12.Value},
		{Path: configValue13.Path, Value: configValue13.Value},
		{Path: configValue14.Path, Value: configValue14.Value},
		{Path: configValue15.Path, Value: configValue15.Value},
	}, nil).AnyTimes()
	mocks.MockStores.DeviceChangesStore.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(device devicetype.VersionedID, c chan<- *devicechange.DeviceChange) (stream.Context, error) {
			go func() {
				c <- deviceChange1
				close(c)
			}()
			return stream.NewContext(func() {

			}), nil
		}).AnyTimes()
}

// setUp should not depend on any global variables
func setUp(t *testing.T) (*Server, *manager.Manager, *AllMocks) {
	var server = &Server{}
	var allMocks AllMocks

	ctrl := gomock.NewController(t)
	mockStores := &mockstore.MockStores{
		DeviceStore:          mockstore.NewMockDeviceStore(ctrl),
		DeviceStateStore:     mockstore.NewMockDeviceStateStore(ctrl),
		NetworkChangesStore:  mockstore.NewMockNetworkChangesStore(ctrl),
		DeviceChangesStore:   mockstore.NewMockDeviceChangesStore(ctrl),
		NetworkSnapshotStore: mockstore.NewMockNetworkSnapshotStore(ctrl),
		DeviceSnapshotStore:  mockstore.NewMockDeviceSnapshotStore(ctrl),
		LeadershipStore:      mockstore.NewMockLeadershipStore(ctrl),
		MastershipStore:      mockstore.NewMockMastershipStore(ctrl),
	}
	deviceCache := mockcache.NewMockCache(ctrl)
	allMocks.MockStores = mockStores
	allMocks.MockDeviceCache = deviceCache

	modelRegistry, err := modelregistry.NewModelRegistry(modelregistry.Config{
		ModPath:      "test/data/" + t.Name() + "/mod",
		RegistryPath: "test/data/" + t.Name() + "/registry",
		PluginPath:   "test/data/" + t.Name() + "/plugins",
		ModTarget:    "github.com/onosproject/onos-config",
	})
	assert.NoError(t, err)

	mgr := manager.NewManager(
		mockStores.LeadershipStore,
		mockStores.MastershipStore,
		mockStores.DeviceChangesStore,
		mockStores.DeviceStateStore,
		mockStores.DeviceStore,
		deviceCache,
		mockStores.NetworkChangesStore,
		mockStores.NetworkSnapshotStore,
		mockStores.DeviceSnapshotStore,
		false,
		modelRegistry)

	mgr.DeviceStore = mockStores.DeviceStore
	mgr.DeviceChangesStore = mockStores.DeviceChangesStore
	mgr.NetworkChangesStore = mockStores.NetworkChangesStore

	log.Infof("Dispatcher pointer %p", &mgr.Dispatcher)
	go listenToTopoLoading(mgr.TopoChannel)
	//go mgr.Dispatcher.Listen(mgr.ChangesChannel)

	setUpWatchMock(&allMocks)
	log.Info("Finished setUp()")

	return server, mgr, &allMocks
}

func setUpBaseNetworkStore(store *mockstore.MockNetworkChangesStore) {
	mockstore.SetUpMapBackedNetworkChangesStore(store)
	now := time.Now()
	config1Value01, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2a", devicechange.NewTypedValueUint(12, 8), false)
	config1Value02, _ := devicechange.NewChangeValue("/cont1a/cont2a/leaf2g", devicechange.NewTypedValueBool(true), false)
	change1 := devicechange.Change{
		Values: []*devicechange.ChangeValue{
			config1Value01, config1Value02},
		DeviceID:      device1,
		DeviceVersion: deviceVersion1,
	}

	networkChange1 := &networkchange.NetworkChange{
		ID:      networkChange1,
		Changes: []*devicechange.Change{&change1},
		Updated: now,
		Created: now,
		Status:  changetypes.Status{State: changetypes.State_COMPLETE},
	}
	_ = store.Create(networkChange1)
}

func setUpBaseDevices(mockStores *mockstore.MockStores, deviceCache *mockcache.MockCache) {
	deviceCache.EXPECT().GetDevicesByID(devicetype.ID("Device1")).Return([]*cache.Info{
		{
			DeviceID: "Device1",
			Version:  "1.0.0",
			Type:     "TestDevice",
		},
	}).AnyTimes()
	deviceCache.EXPECT().GetDevices().Return([]*cache.Info{
		{
			DeviceID: "Device1",
			Version:  "1.0.0",
			Type:     "TestDevice",
		},
		{
			DeviceID: "Device2",
			Version:  "2.0.0",
			Type:     "TestDevice",
		},
		{
			DeviceID: "Device3",
			Version:  "1.0.0",
			Type:     "TestDevice",
		},
	}).AnyTimes()

	mockStores.DeviceStore.EXPECT().Get(topodevice.ID(device1)).Return(nil, status.Error(codes.NotFound, "device not found")).AnyTimes()
	mockStores.DeviceStore.EXPECT().List(gomock.Any()).DoAndReturn(
		func(c chan<- *topodevice.Device) (stream.Context, error) {
			go func() {
				c <- &topodevice.Device{
					ID:      "Device1 (1.0.0)",
					Type:    "TestDevice",
					Version: "1.0.0",
				}
				c <- &topodevice.Device{
					ID:      "Device2 (2.0.0)",
					Type:    "TestDevice",
					Version: "2.0.0",
				}
				c <- &topodevice.Device{
					ID:      "Device3",
					Type:    "TestDevice",
					Version: "1.0.0",
				}
				close(c)
			}()
			return stream.NewContext(func() {
			}), nil
		}).AnyTimes()
}

type MockModel struct {
	schemaFn func() (map[string]*yang.Entry, error)
}

func (m MockModel) Info() configmodel.ModelInfo {
	panic("implement me")
}

func (m MockModel) Data() []*gnmi.ModelData {
	panic("implement me")
}

func (m MockModel) Schema() (map[string]*yang.Entry, error) {
	return m.schemaFn()
}

func (m MockModel) GetStateMode() configmodel.GetStateMode {
	panic("implement me")
}

func (m MockModel) Unmarshaler() configmodel.Unmarshaler {
	return func(bytes []byte) (*ygot.ValidatedGoStruct, error) {
		device := &td1.Device{}
		vgs := ygot.ValidatedGoStruct(device)
		if err := td1.Unmarshal(bytes, device); err != nil {
			return nil, err
		}
		return &vgs, nil
	}
}

func (m MockModel) Validator() configmodel.Validator {
	return func(model *ygot.ValidatedGoStruct, opts ...ygot.ValidationOption) error {
		deviceDeref := *model
		device, ok := deviceDeref.(*td1.Device)
		if !ok {
			return fmt.Errorf("unable to convert model in to testdevice_1_0_0")
		}
		return device.Validate()
	}
}

func setUpForGetSetTests(t *testing.T) (*Server, *AllMocks, *manager.Manager) {
	server, mgr, allMocks := setUp(t)
	allMocks.MockStores.NetworkChangesStore = mockstore.NewMockNetworkChangesStore(gomock.NewController(t))
	mgr.NetworkChangesStore = allMocks.MockStores.NetworkChangesStore
	setUpBaseNetworkStore(allMocks.MockStores.NetworkChangesStore)
	setUpBaseDevices(allMocks.MockStores, allMocks.MockDeviceCache)
	//setUpChangesMock(allMocks)
	modelSchema, err := td1.UnzipSchema()
	assert.NoError(t, err)
	_, modelRwPaths := modelregistry.ExtractPaths(modelSchema["Device"], yang.TSUnset, "", "")
	modelPlugin := &modelregistry.ModelPlugin{
		Info: configmodel.ModelInfo{
			Name:    "TestDevice",
			Version: "1.0.0",
		},
		Model:          MockModel{},
		ReadWritePaths: modelRwPaths,
	}
	config := modelregistry.Config{
		ModPath:      "test/data/" + t.Name() + "/mod",
		RegistryPath: "test/data/" + t.Name() + "/registry",
		PluginPath:   "test/data/" + t.Name() + "/plugins",
		ModTarget:    "github.com/onosproject/onos-config@master",
	}
	modelRegistry, err := modelregistry.NewModelRegistry(config, modelPlugin)
	assert.NoError(t, err)
	mgr.ModelRegistry = modelRegistry
	return server, allMocks, mgr
}

func setUpPathsForGetSetTests() ([]*gnmi.Path, []*gnmi.Update, []*gnmi.Update) {
	var deletePaths = make([]*gnmi.Path, 0)
	var replacedPaths = make([]*gnmi.Update, 0)
	var updatedPaths = make([]*gnmi.Update, 0)
	return deletePaths, replacedPaths, updatedPaths
}

func tearDown(mgr *manager.Manager, wg *sync.WaitGroup) {
	// `wg.Wait` blocks until `wg.Done` is called the same number of times
	// as the amount of tasks we have (in this case, 1 time)
	wg.Wait()

	mgr.Dispatcher = &dispatcher.Dispatcher{}
	log.Infof("Dispatcher Teardown %p", mgr.Dispatcher)

}

func listenToTopoLoading(deviceChan <-chan *topodevice.ListResponse) {
	for deviceConfigEvent := range deviceChan {
		log.Info("Ignoring event for testing ", deviceConfigEvent)
	}
}
