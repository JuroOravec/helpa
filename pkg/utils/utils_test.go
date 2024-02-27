package utils

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

type TestStruct struct {
	Name      string
	NumOfEggs float64
}

type TestStructNested struct {
	TestStruct

	City  string
	AList []string
}

func TestApplyDefaults(t *testing.T) {
	assert := assert.New(t)

	myStruct := TestStruct{}
	defaults := TestStruct{NumOfEggs: 1.4}

	err := ApplyDefaults(&myStruct, defaults)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(myStruct.Name, "")
	assert.Equal(myStruct.NumOfEggs, 1.4)
}

func TestApplyDefaultsZero(t *testing.T) {
	assert := assert.New(t)

	myStruct := TestStruct{}
	defaults := TestStruct{}

	err := ApplyDefaults(&myStruct, defaults)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(myStruct.Name, "")
	assert.Equal(myStruct.NumOfEggs, 0.0)
}

func TestApplyDefaultsNested(t *testing.T) {
	assert := assert.New(t)

	myStruct := TestStructNested{City: "Berlin"}
	defaults := TestStructNested{AList: []string{"Apple"}, TestStruct: TestStruct{NumOfEggs: 2}}

	err := ApplyDefaults(&myStruct, &defaults)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(myStruct.AList, []string{"Apple"})
	assert.Equal(myStruct.City, "Berlin")
	assert.Equal(myStruct.Name, "")
	assert.Equal(myStruct.NumOfEggs, 2.0)
}
