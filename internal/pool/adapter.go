package pool

import (
	"xiaozhi-esp32-server-golang/internal/util"
)

// ResourceWrapper 泛型resourcepackage装器
// T: concreteofresourcetype（如 vad.VAD, asr.AsrProvider etc）
type ResourceWrapper[T any] struct {
	provider     T                    // actualofresourceprovide者（type安全）
	configKey    string               // configkey，used for标识resourcepool
	resourceType string               // resourcetype（vad/asr/llm/ttsetc）
	closeFunc    func(T) error        // closeresourceoffunction
	isValidFunc  func(T) bool         // validateresourcewhethervalidoffunction
	resetFunc    func(T) error        // resetresourcestateoffunction（optional）
}

// Close closeresource
func (r *ResourceWrapper[T]) Close() error {
	if r.closeFunc != nil {
		return r.closeFunc(r.provider)
	}
	return nil
}

// IsValid inspectresourcewhethervalid
func (r *ResourceWrapper[T]) IsValid() bool {
	if r.isValidFunc != nil {
		return r.isValidFunc(r.provider)
	}
	var zero T
	return any(r.provider) != any(zero)
}

// GetProvider getactualofresourceprovide者（type安全，noneedtypeassert）
func (r *ResourceWrapper[T]) GetProvider() T {
	return r.provider
}

// GetConfigKey getconfigkey
func (r *ResourceWrapper[T]) GetConfigKey() string {
	return r.configKey
}

// GetResourceType getresourcetype
func (r *ResourceWrapper[T]) GetResourceType() string {
	return r.resourceType
}

// Reset resetresourcestate
func (r *ResourceWrapper[T]) Reset() error {
	if r.resetFunc != nil {
		return r.resetFunc(r.provider)
	}
	return nil
}

// CreatorFunc 泛型resourcecreatefunctiontype
// T: resourcetype
// parameter：resourceType, provider, config
// return：resourceinstance（type T）anderror
type CreatorFunc[T any] func(resourceType, provider string, config map[string]interface{}) (T, error)

// ResourceFactory 泛型resourcefactory
type ResourceFactory[T any] struct {
	resourceType string
	provider     string
	config       map[string]interface{}
	configKey    string
	creator      CreatorFunc[T]
	closeFunc    func(T) error
	isValidFunc  func(T) bool
	resetFunc    func(T) error
}

// Create createresource
func (f *ResourceFactory[T]) Create() (util.Resource, error) {
	provider, err := f.creator(f.resourceType, f.provider, f.config)
	if err != nil {
		return nil, err
	}

	return &ResourceWrapper[T]{
		provider:     provider,
		configKey:    f.configKey,
		resourceType: f.resourceType,
		closeFunc:    f.closeFunc,
		isValidFunc:  f.isValidFunc,
		resetFunc:    f.resetFunc,
	}, nil
}

// Validate validateresource
func (f *ResourceFactory[T]) Validate(resource util.Resource) bool {
	if wrapper, ok := resource.(*ResourceWrapper[T]); ok {
		if f.isValidFunc != nil {
			return f.isValidFunc(wrapper.provider)
		}
		return wrapper.IsValid()
	}
	return resource != nil && resource.IsValid()
}

// Reset resetresource
func (f *ResourceFactory[T]) Reset(resource util.Resource) error {
	if wrapper, ok := resource.(*ResourceWrapper[T]); ok {
		if wrapper.resetFunc != nil {
			return wrapper.resetFunc(wrapper.provider)
		}
		return wrapper.Reset()
	}
	return nil
}
