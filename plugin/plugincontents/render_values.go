package plugincontents

import (
	"fmt"
	"reflect"

	"github.com/davecgh/go-spew/spew"
)

type valuesRenderer struct {
	values map[string]interface{}
	config interface{}
}

func RenderTemplatesInValues(name string, values map[string]interface{}, config interface{}) (map[string]interface{}, error) {
	r := valuesRenderer{
		values: values,
		config: config,
	}

	rendered, err := r.translate()
	if err != nil {
		return nil, fmt.Errorf("failed to render values for plugin %s: %v", name, err)
	}
	return rendered, nil
}

func (r valuesRenderer) translate() (map[string]interface{}, error) {
	// Wrap the original in a reflect.Value
	original := reflect.ValueOf(r.values)

	copy := reflect.New(original.Type()).Elem()
	if err := r.translateRecursive(copy, original); err != nil {
		return nil, fmt.Errorf("failed to translate values: %v", err)
	}

	// Remove the reflection wrapper
	return copy.Interface().(map[string]interface{}), nil
}

func (r valuesRenderer) translateRecursive(copy, original reflect.Value) error {
	switch original.Kind() {
	// The first cases handle nested structures and translate them recursively

	// If it is a pointer we need to unwrap and call once again
	case reflect.Ptr:
		// To get the actual value of the original we have to call Elem()
		// At the same time this unwraps the pointer so we don't end up in
		// an infinite recursion
		originalValue := original.Elem()
		// Check if the pointer is nil
		if !originalValue.IsValid() {
			return nil
		}
		// Allocate a new object and set the pointer to it
		copy.Set(reflect.New(originalValue.Type()))
		// Unwrap the newly created pointer
		if err := r.translateRecursive(copy.Elem(), originalValue); err != nil {
			return err
		}

	// If it is an interface (which is very similar to a pointer), do basically the
	// same as for the pointer. Though a pointer is not the same as an interface so
	// note that we have to call Elem() after creating a new object because otherwise
	// we would end up with an actual pointer
	case reflect.Interface:
		// Get rid of the wrapping interface
		originalValue := original.Elem()
		// Create a new object. Now new gives us a pointer, but we want the value it
		// points to, so we have to call Elem() to unwrap it
		copyValue := reflect.New(originalValue.Type()).Elem()
		if err := r.translateRecursive(copyValue, originalValue); err != nil {
			return err
		}
		copy.Set(copyValue)

	// If it is a struct we translate each field
	case reflect.Struct:
		for i := 0; i < original.NumField(); i += 1 {
			if err := r.translateRecursive(copy.Field(i), original.Field(i)); err != nil {
				return err
			}
		}

	// If it is a slice we create a new slice and translate each element
	case reflect.Slice:
		copy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i += 1 {
			if err := r.translateRecursive(copy.Index(i), original.Index(i)); err != nil {
				return err
			}
		}

	// If it is a map we create a new map and translate each value
	case reflect.Map:
		copy.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			// New gives us a pointer, but again we want the value
			copyValue := reflect.New(originalValue.Type()).Elem()
			if err := r.translateRecursive(copyValue, originalValue); err != nil {
				return err
			}
			copy.SetMapIndex(key, copyValue)
		}

	// Otherwise we cannot traverse anywhere so this finishes the the recursion

	// If it is a string translate it (yay finally we're doing what we came for)
	// check whether the string looks like a template and replace it with the rendered version if it is
	// note1: string is rendered as a template with a datacontext as saved in the values renderer
	// note2: we are unable to use other values that themselves contain templates - too complicated!
	case reflect.String:
		var isTemplate bool
		var err error
		stringValue := original.Interface().(string)
		if isTemplate, err = LooksLikeATemplate(stringValue); err != nil {
			return err
		}
		if isTemplate {
			var renderedString string
			if renderedString, err = RenderStringFromTemplateWithValues(stringValue, r.values, r.config); err != nil {
				spew.Dump(r.config)
				return fmt.Errorf("failed to render values template string '%s': %v", stringValue, err)
			}
			stringValue = renderedString
		}
		copy.SetString(stringValue)

	// And everything else will simply be taken from the original
	default:
		copy.Set(original)
	}

	return nil
}
