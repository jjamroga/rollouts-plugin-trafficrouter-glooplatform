package util

import (
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CustomResourceWrapper holds an object of type client.Object
// Functions other than `New` or `Build` must implement method chaining
type CustomResourceWrapper[T client.Object] struct {
	t T
}

// NewObject will accept a struct implementing client.Object
// GVK must be set on the object being passed in order to support patches
func NewObject[T client.Object](obj T) *CustomResourceWrapper[T] {
	asClientObject := any(obj).(client.Object)
	if asClientObject.GetObjectKind().GroupVersionKind().Empty() {
		panic("Object does not have GVK set")
	}
	if obj.GetName() == "" {
		obj.SetName(uuid.New().String())
	}
	if obj.GetNamespace() == "" {
		obj.SetNamespace(uuid.New().String())
	}

	return &CustomResourceWrapper[T]{t: obj}
}

// Build will return the object of expected type T
func (c *CustomResourceWrapper[T]) Build() T {
	return c.t
}

// Modify will allow a copy of the object to be manipulated with a callback function.
// *Does not* modify the original object.
func (c *CustomResourceWrapper[T]) Modify(modify func(obj T)) *CustomResourceWrapper[T] {
	object := c.Build().DeepCopyObject().(T)
	if modify != nil {
		modify(object)
	}
	return NewObject[T](object)
}

// FromYaml allows an object to be created from a YAML string
func FromYaml[T client.Object](yamlStr string) *CustomResourceWrapper[T] {
	var target T
	err := yaml.Unmarshal([]byte(yamlStr), &target)
	if err != nil {
		// We want to immediate fail if we can't parse the YAML
		panic(fmt.Sprintf("%s: %s", reflect.TypeOf(target), err.Error()))
	}
	return NewObject[T](target)
}
