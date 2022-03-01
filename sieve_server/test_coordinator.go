package main

import (
	"log"
	"sync"

	sieve "sieve.client"
)

type TestCoordinator struct {
	Server *testCoordinator
}

type ActionContext struct {
	namespace                 string
	leadingAPIServer          string
	followingAPIServer        string
	apiserverLocks            map[string]map[string]chan string
	apiserverLockedMap        map[string]map[string]bool
	controllerOngoingReadLock *sync.Mutex
	asyncDoneCh               chan *AsyncDoneNotification
}

type testCoordinator struct {
	testPlan                   *TestPlan
	actionConext               *ActionContext
	stateNotificationCh        chan TriggerNotification
	objectStates               map[string]map[string]map[string]string
	objectStatesLock           sync.RWMutex
	controllerOngoingReadLock  *sync.Mutex
	apiserverLocks             map[string]map[string]chan string
	apiserverLockedMap         map[string]map[string]bool
	mergedFieldPathMask        map[string]map[string]struct{}
	mergedFieldKeyMask         map[string]map[string]struct{}
	mergedFieldPathMaskAPIFrom map[string]map[string]struct{}
	mergedFieldKeyMaskAPIForm  map[string]map[string]struct{}
	stateMachine               *StateMachine
}

func NewTestCoordinator() *TestCoordinator {
	config := getConfig()
	testPlan := parseTestPlan(config)
	asyncDoneCh := make(chan *AsyncDoneNotification)
	controllerOngoingReadLock := &sync.Mutex{}
	apiserverLocks := make(map[string]map[string]chan string)
	apiserverLockedMap := make(map[string]map[string]bool)
	actionConext := &ActionContext{
		namespace:                 "default",
		leadingAPIServer:          "kind-control-plane",
		followingAPIServer:        "kind-control-plane3",
		apiserverLocks:            apiserverLocks,
		apiserverLockedMap:        apiserverLockedMap,
		controllerOngoingReadLock: controllerOngoingReadLock,
		asyncDoneCh:               asyncDoneCh,
	}
	mergedFieldPathMask, mergedFieldKeyMask := getMergedMask()
	stateNotificationCh := make(chan TriggerNotification, 500)
	server := &testCoordinator{
		testPlan:                   testPlan,
		actionConext:               actionConext,
		stateNotificationCh:        stateNotificationCh,
		objectStates:               map[string]map[string]map[string]string{},
		controllerOngoingReadLock:  controllerOngoingReadLock,
		apiserverLocks:             apiserverLocks,
		apiserverLockedMap:         apiserverLockedMap,
		mergedFieldPathMask:        mergedFieldPathMask,
		mergedFieldKeyMask:         mergedFieldKeyMask,
		mergedFieldPathMaskAPIFrom: convertFieldPathMaskToAPIForm(mergedFieldPathMask),
		mergedFieldKeyMaskAPIForm:  convertFieldKeyMaskToAPIForm(mergedFieldKeyMask),
		stateMachine:               NewStateMachine(testPlan, stateNotificationCh, asyncDoneCh, actionConext),
	}
	listener := &TestCoordinator{
		Server: server,
	}
	listener.Server.Start()
	return listener
}

func (tc *TestCoordinator) NotifyTestBeforeAPIServerRecv(request *sieve.NotifyTestBeforeAPIServerRecvRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestBeforeAPIServerRecv(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterAPIServerRecv(request *sieve.NotifyTestAfterAPIServerRecvRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterAPIServerRecv(request, response)
}

func (tc *TestCoordinator) NotifyTestBeforeControllerRecv(request *sieve.NotifyTestBeforeControllerRecvRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestBeforeControllerRecv(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterControllerRecv(request *sieve.NotifyTestAfterControllerRecvRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterControllerRecv(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterControllerGet(request *sieve.NotifyTestAfterControllerGetRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterControllerGet(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterControllerList(request *sieve.NotifyTestAfterControllerListRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterControllerList(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterControllerWrite(request *sieve.NotifyTestAfterControllerWriteRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterControllerWrite(request, response)
}

func (tc *TestCoordinator) NotifyTestBeforeControllerReadPause(request *sieve.NotifyTestBeforeControllerReadPauseRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestBeforeControllerReadPause(request, response)
}

func (tc *TestCoordinator) NotifyTestAfterControllerReadPause(request *sieve.NotifyTestAfterControllerReadPauseRequest, response *sieve.Response) error {
	return tc.Server.NotifyTestAfterControllerReadPause(request, response)
}

func (tc *testCoordinator) Start() {
	log.Println("start testCoordinator...")
	log.Printf("mergedFieldPathMask:\n%v\n", tc.mergedFieldPathMask)
	log.Printf("mergedFieldKeyMask:\n%v\n", tc.mergedFieldKeyMask)
	log.Printf("mergedFieldPathMaskAPIFrom:\n%v\n", tc.mergedFieldPathMaskAPIFrom)
	log.Printf("mergedFieldKeyMaskAPIForm:\n%v\n", tc.mergedFieldKeyMaskAPIForm)
	go tc.stateMachine.run()
}

func (tc *testCoordinator) SendObjectCreateNotificationAndBlock(handlerName, resourceKey, observedWhen, observedBy string) {
	blockingCh := make(chan string)
	notification := &ObjectCreateNotification{
		resourceKey:  resourceKey,
		observedWhen: observedWhen,
		observedBy:   observedBy,
		blockingCh:   blockingCh,
	}
	log.Printf("%s: send ObjectCreateNotification\n", handlerName)
	tc.stateNotificationCh <- notification
	<-blockingCh
	log.Printf("%s: block over for ObjectCreateNotification\n", handlerName)
}

func (tc *testCoordinator) SendObjectDeleteNotificationAndBlock(handlerName, resourceKey, observedWhen, observedBy string) {
	blockingCh := make(chan string)
	notification := &ObjectDeleteNotification{
		resourceKey:  resourceKey,
		observedWhen: observedWhen,
		observedBy:   observedBy,
		blockingCh:   blockingCh,
	}
	log.Printf("%s: send ObjectDeleteNotification\n", handlerName)
	tc.stateNotificationCh <- notification
	<-blockingCh
	log.Printf("%s: block over for ObjectDeleteNotification\n", handlerName)
}

func (tc *testCoordinator) SendObjectUpdateNotificationAndBlock(handlerName, resourceKey, observedWhen, observedBy string, prevState, curState map[string]interface{}) {
	blockingCh := make(chan string)
	notification := &ObjectUpdateNotification{
		resourceKey:          resourceKey,
		observedWhen:         observedWhen,
		observedBy:           observedBy,
		prevState:            prevState,
		curState:             curState,
		fieldKeyMask:         getMaskByResourceKey(tc.mergedFieldKeyMask, resourceKey),
		fieldPathMask:        getMaskByResourceKey(tc.mergedFieldPathMask, resourceKey),
		fieldKeyMaskAPIForm:  getMaskByResourceKey(tc.mergedFieldKeyMaskAPIForm, resourceKey),
		fieldPathMaskAPIForm: getMaskByResourceKey(tc.mergedFieldPathMaskAPIFrom, resourceKey),
		blockingCh:           blockingCh,
	}
	log.Printf("%s: send ObjectUpdateNotification\n", handlerName)
	tc.stateNotificationCh <- notification
	<-blockingCh
	log.Printf("%s: block over for ObjectUpdateNotification\n", handlerName)
}

func (tc *testCoordinator) InitializeObjectStatesEntry(observedBy, observedWhen, resourceKey string) {
	tc.objectStatesLock.Lock()
	defer tc.objectStatesLock.Unlock()
	if _, ok := tc.objectStates[observedBy]; !ok {
		tc.objectStates[observedBy] = map[string]map[string]string{}
	}
	if _, ok := tc.objectStates[observedBy][observedWhen]; !ok {
		tc.objectStates[observedBy][observedWhen] = map[string]string{}
	}
	if _, ok := tc.objectStates[observedBy][observedWhen][resourceKey]; !ok {
		tc.objectStates[observedBy][observedWhen][resourceKey] = "{}"
	}
}

func (tc *testCoordinator) ReadFromObjectStates(observedBy, observedWhen, resourceKey string) string {
	tc.objectStatesLock.RLock()
	defer tc.objectStatesLock.RUnlock()
	return tc.objectStates[observedBy][observedWhen][resourceKey]
}

func (tc *testCoordinator) WriteToObjectStates(observedBy, observedWhen, resourceKey string, value string) {
	tc.objectStatesLock.Lock()
	defer tc.objectStatesLock.Unlock()
	tc.objectStates[observedBy][observedWhen][resourceKey] = value
}

func (tc *testCoordinator) ProcessControllerReadStartNotification() {
	tc.controllerOngoingReadLock.Lock()
}

func (tc *testCoordinator) ProcessControllerReadEndNotification() {
	tc.controllerOngoingReadLock.Unlock()
}

func (tc *testCoordinator) ProcessAPIServerRecvNotification(handlerName, apiServerHostname, pauseScope string) {
	if _, ok := tc.apiserverLockedMap[apiServerHostname]; ok {
		if val, ok := tc.apiserverLockedMap[apiServerHostname][pauseScope]; ok {
			if val {
				log.Printf("Start to pause API server for %s\n", apiServerHostname)
				pausingCh := tc.apiserverLocks[apiServerHostname][pauseScope]
				<-pausingCh
				return
			}
		}
		if val, ok := tc.apiserverLockedMap[apiServerHostname]["all"]; ok {
			if val {
				log.Printf("Start to pause API server for %s, %s\n", apiServerHostname, pauseScope)
				pausingCh := tc.apiserverLocks[apiServerHostname]["all"]
				<-pausingCh
				return
			}
		}
	}
}

func (tc *testCoordinator) NotifyTestBeforeAPIServerRecv(request *sieve.NotifyTestBeforeAPIServerRecvRequest, response *sieve.Response) error {
	handlerName := "NotifyTestBeforeAPIServerRecv"
	log.Printf("%s\t%s\t%s\t%s\t%s", request.APIServerHostname, handlerName, request.OperationType, request.ResourceKey, request.Object)
	tc.InitializeObjectStatesEntry(request.APIServerHostname, beforeAPIServerRecv, request.ResourceKey)
	switch request.OperationType {
	case API_ADDED:
		tc.SendObjectCreateNotificationAndBlock(handlerName, request.ResourceKey, beforeAPIServerRecv, request.APIServerHostname)
	case API_MODIFIED:
		prevObjectStateStr := tc.ReadFromObjectStates(request.APIServerHostname, beforeAPIServerRecv, request.ResourceKey)
		tc.SendObjectUpdateNotificationAndBlock(handlerName, request.ResourceKey, beforeAPIServerRecv, request.APIServerHostname, strToMap(prevObjectStateStr), strToMap(request.Object))
	case API_DELETED:
		tc.SendObjectDeleteNotificationAndBlock(handlerName, request.ResourceKey, beforeAPIServerRecv, request.APIServerHostname)
	default:
		log.Printf("do not support %s\n", request.OperationType)
	}
	tc.WriteToObjectStates(request.APIServerHostname, beforeAPIServerRecv, request.ResourceKey, request.Object)
	tc.ProcessAPIServerRecvNotification(handlerName, request.APIServerHostname, request.ResourceKey)
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterAPIServerRecv(request *sieve.NotifyTestAfterAPIServerRecvRequest, response *sieve.Response) error {
	handlerName := "NotifyTestAfterAPIServerRecv"
	log.Printf("%s\t%s\t%s\t%s\t%s", request.APIServerHostname, handlerName, request.OperationType, request.ResourceKey, request.Object)
	tc.InitializeObjectStatesEntry(request.APIServerHostname, afterAPIServerRecv, request.ResourceKey)
	switch request.OperationType {
	case API_ADDED:
		tc.SendObjectCreateNotificationAndBlock(handlerName, request.ResourceKey, afterAPIServerRecv, request.APIServerHostname)
	case API_MODIFIED:
		prevObjectStateStr := tc.ReadFromObjectStates(request.APIServerHostname, afterAPIServerRecv, request.ResourceKey)
		tc.SendObjectUpdateNotificationAndBlock(handlerName, request.ResourceKey, afterAPIServerRecv, request.APIServerHostname, strToMap(prevObjectStateStr), strToMap(request.Object))
	case API_DELETED:
		tc.SendObjectDeleteNotificationAndBlock(handlerName, request.ResourceKey, afterAPIServerRecv, request.APIServerHostname)
	default:
		log.Printf("do not support %s\n", request.OperationType)
	}
	tc.WriteToObjectStates(request.APIServerHostname, afterAPIServerRecv, request.ResourceKey, request.Object)
	tc.ProcessAPIServerRecvNotification(handlerName, request.APIServerHostname, request.ResourceKey)
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestBeforeControllerRecv(request *sieve.NotifyTestBeforeControllerRecvRequest, response *sieve.Response) error {
	handlerName := "NotifyTestBeforeControllerRecv"
	log.Printf("%s\t%s\t%s\t%s", handlerName, request.OperationType, request.ResourceKey, request.Object)
	tc.InitializeObjectStatesEntry("informer", beforeControllerRecv, request.ResourceKey)
	switch request.OperationType {
	case HEAR_ADDED:
		tc.SendObjectCreateNotificationAndBlock(handlerName, request.ResourceKey, beforeControllerRecv, "informer")
	case HEAR_UPDATED, HEAR_REPLACED, HEAR_SYNC:
		prevObjectStateStr := tc.ReadFromObjectStates("informer", beforeControllerRecv, request.ResourceKey)
		tc.SendObjectUpdateNotificationAndBlock(handlerName, request.ResourceKey, beforeControllerRecv, "informer", strToMap(prevObjectStateStr), strToMap(request.Object))
	case HEAR_DELETED:
		tc.SendObjectDeleteNotificationAndBlock(handlerName, request.ResourceKey, beforeControllerRecv, "informer")
	default:
		log.Printf("do not support %s\n", request.OperationType)
	}
	tc.WriteToObjectStates("informer", beforeControllerRecv, request.ResourceKey, request.Object)
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterControllerRecv(request *sieve.NotifyTestAfterControllerRecvRequest, response *sieve.Response) error {
	handlerName := "NotifyTestAfterControllerRecv"
	log.Printf("%s\t%s\t%s\t%s", handlerName, request.OperationType, request.ResourceKey, request.Object)
	tc.InitializeObjectStatesEntry("informer", afterControllerRecv, request.ResourceKey)
	switch request.OperationType {
	case HEAR_ADDED:
		tc.SendObjectCreateNotificationAndBlock(handlerName, request.ResourceKey, afterControllerRecv, "informer")
	case HEAR_UPDATED, HEAR_REPLACED, HEAR_SYNC:
		prevObjectStateStr := tc.ReadFromObjectStates("informer", afterControllerRecv, request.ResourceKey)
		tc.SendObjectUpdateNotificationAndBlock(handlerName, request.ResourceKey, afterControllerRecv, "informer", strToMap(prevObjectStateStr), strToMap(request.Object))
	case HEAR_DELETED:
		tc.SendObjectDeleteNotificationAndBlock(handlerName, request.ResourceKey, afterControllerRecv, "informer")
	default:
		log.Printf("do not support %s\n", request.OperationType)
	}
	tc.WriteToObjectStates("informer", beforeControllerRecv, request.ResourceKey, request.Object)
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterControllerGet(request *sieve.NotifyTestAfterControllerGetRequest, response *sieve.Response) error {
	log.Printf("NotifyTestAfterControllerGet\t%s\t%s\t%s", request.ResourceKey, request.ReconcilerType, request.Object)
	tc.InitializeObjectStatesEntry(request.ReconcilerType, afterControllerWrite, request.ResourceKey)
	tc.WriteToObjectStates(request.ReconcilerType, afterControllerWrite, request.ResourceKey, request.Object)
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterControllerList(request *sieve.NotifyTestAfterControllerListRequest, response *sieve.Response) error {
	log.Printf("NotifyTestAfterControllerList\t%s\t%s\t%s", request.ResourceType, request.ReconcilerType, request.ObjectList)
	objects := strToMap(request.ObjectList)["items"].([]interface{})
	for _, object := range objects {
		objectState := object.(map[string]interface{})
		name, namespace := extractNameNamespaceFromObjMap(objectState)
		resourceKey := generateResourceKey(request.ResourceType, namespace, name)
		tc.InitializeObjectStatesEntry(request.ReconcilerType, afterControllerWrite, resourceKey)
		tc.WriteToObjectStates(request.ReconcilerType, afterControllerWrite, resourceKey, mapToStr(objectState))
	}
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterControllerWrite(request *sieve.NotifyTestAfterControllerWriteRequest, response *sieve.Response) error {
	handlerName := "NotifyTestAfterControllerWrite"
	log.Printf("%s\t%s\t%s\t%s\t%s", handlerName, request.WriteType, request.ResourceKey, request.ReconcilerType, request.Object)
	tc.InitializeObjectStatesEntry(request.ReconcilerType, afterControllerWrite, request.ResourceKey)
	prevObjectStateStr := tc.ReadFromObjectStates(request.ReconcilerType, afterControllerWrite, request.ResourceKey)
	switch request.WriteType {
	case WRITE_CREATE:
		tc.SendObjectCreateNotificationAndBlock(handlerName, request.ResourceKey, afterControllerWrite, request.ReconcilerType)
	case WRITE_UPDATE, WRITE_PATCH, WRITE_STATUS_UPDATE, WRITE_STATUS_PATCH:
		prevObjectState := strToMap(prevObjectStateStr)
		trimKindApiversion(prevObjectState)
		curObjectState := strToMap(request.Object)
		trimKindApiversion(curObjectState)
		tc.SendObjectUpdateNotificationAndBlock(handlerName, request.ResourceKey, afterControllerWrite, request.ReconcilerType, prevObjectState, curObjectState)
	case WRITE_DELETE:
		tc.SendObjectDeleteNotificationAndBlock(handlerName, request.ResourceKey, afterControllerWrite, request.ReconcilerType)
	default:
		log.Printf("do not support %s\n", request.WriteType)
	}
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestBeforeControllerReadPause(request *sieve.NotifyTestBeforeControllerReadPauseRequest, response *sieve.Response) error {
	handlerName := "NotifyTestBeforeControllerReadPause"
	log.Printf("%s\t%t\t%s\t%s", handlerName, request.UseResourceKey, request.ResourceKey, request.ResourceType)
	tc.ProcessControllerReadStartNotification()
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}

func (tc *testCoordinator) NotifyTestAfterControllerReadPause(request *sieve.NotifyTestAfterControllerReadPauseRequest, response *sieve.Response) error {
	handlerName := "NotifyTestAfterControllerReadPause"
	log.Printf("%s\t%t\t%s\t%s", handlerName, request.UseResourceKey, request.ResourceKey, request.ResourceType)
	tc.ProcessControllerReadEndNotification()
	*response = sieve.Response{Message: "", Ok: true}
	return nil
}
