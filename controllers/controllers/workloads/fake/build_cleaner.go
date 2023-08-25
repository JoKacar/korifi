// Code generated by counterfeiter. DO NOT EDIT.
package fake

import (
	"context"
	"sync"

	"code.cloudfoundry.org/korifi/controllers/controllers/workloads/build"
	"k8s.io/apimachinery/pkg/types"
)

type BuildCleaner struct {
	CleanStub        func(context.Context, types.NamespacedName) error
	cleanMutex       sync.RWMutex
	cleanArgsForCall []struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}
	cleanReturns struct {
		result1 error
	}
	cleanReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *BuildCleaner) Clean(arg1 context.Context, arg2 types.NamespacedName) error {
	fake.cleanMutex.Lock()
	ret, specificReturn := fake.cleanReturnsOnCall[len(fake.cleanArgsForCall)]
	fake.cleanArgsForCall = append(fake.cleanArgsForCall, struct {
		arg1 context.Context
		arg2 types.NamespacedName
	}{arg1, arg2})
	stub := fake.CleanStub
	fakeReturns := fake.cleanReturns
	fake.recordInvocation("Clean", []interface{}{arg1, arg2})
	fake.cleanMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *BuildCleaner) CleanCallCount() int {
	fake.cleanMutex.RLock()
	defer fake.cleanMutex.RUnlock()
	return len(fake.cleanArgsForCall)
}

func (fake *BuildCleaner) CleanCalls(stub func(context.Context, types.NamespacedName) error) {
	fake.cleanMutex.Lock()
	defer fake.cleanMutex.Unlock()
	fake.CleanStub = stub
}

func (fake *BuildCleaner) CleanArgsForCall(i int) (context.Context, types.NamespacedName) {
	fake.cleanMutex.RLock()
	defer fake.cleanMutex.RUnlock()
	argsForCall := fake.cleanArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *BuildCleaner) CleanReturns(result1 error) {
	fake.cleanMutex.Lock()
	defer fake.cleanMutex.Unlock()
	fake.CleanStub = nil
	fake.cleanReturns = struct {
		result1 error
	}{result1}
}

func (fake *BuildCleaner) CleanReturnsOnCall(i int, result1 error) {
	fake.cleanMutex.Lock()
	defer fake.cleanMutex.Unlock()
	fake.CleanStub = nil
	if fake.cleanReturnsOnCall == nil {
		fake.cleanReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *BuildCleaner) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.cleanMutex.RLock()
	defer fake.cleanMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *BuildCleaner) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ build.BuildCleaner = new(BuildCleaner)
