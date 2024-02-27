package utils

import (
	"fmt"
	"reflect"

	reflections "github.com/oleiade/reflections"
)

// See https://stackoverflow.com/a/49471736/9788634
func ApplyDefaults(s any, defaults any) error {
	if s == nil {
		return nil
	}

	defFieldValues, err := reflections.Items(defaults)
	if err != nil {
		return err
	}
	fieldNames, err := reflections.Fields(s)
	if err != nil {
		return err
	}

	val := reflect.ValueOf(s)

	// If it's an interface or a pointer, unwrap it.
	if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct {
		val = val.Elem()
	} else {
		return fmt.Errorf("s must be a struct")
	}

	valNumFields := val.NumField()

	for i := 0; i < valNumFields; i++ {
		fieldName := fieldNames[i]

		field := val.Field(i)
		fieldKind := field.Kind()
		dftField := reflect.ValueOf(defFieldValues[fieldName])

		// Check if it's a pointer to a struct.
		if fieldKind == reflect.Ptr && field.Elem().Kind() == reflect.Struct {
			if field.CanInterface() {
				// Recurse using an interface of the field.
				err := ApplyDefaults(field.Interface(), dftField.Interface())
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		// Check if it's a struct value.
		if fieldKind == reflect.Struct {
			if field.CanAddr() && field.Addr().CanInterface() {
				// Recurse using an interface of the pointer value of the field.
				err := ApplyDefaults(
					field.Addr().Interface(),
					defFieldValues[fieldName],
				)
				if err != nil {
					return err
				}
			}

			// Move onto the next field.
			continue
		}

		// Do nothing if the value is set
		isZero := field.IsZero()
		if !isZero {
			continue
		}

		reflections.SetField(s, fieldNames[i], defFieldValues[fieldNames[i]])
	}

	return nil
}
