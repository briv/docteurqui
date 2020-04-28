package censor

import (
	"bytes"
	"testing"
)

const StringWithPersonalData = "very personal data"

func mkKey(n int) []byte {
	return make([]byte, n)
}

func smallKey() []byte { return mkKey(12) }
func goodKey() []byte  { return mkKey(MinKeySize) }
func bigKey() []byte   { return mkKey(500) }

func TestNoInitialization(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
		} else {
			t.Error("no panic detected")
		}
	}()

	Censor(StringWithPersonalData)
}

func TestBadInitialization(t *testing.T) {
	key := smallKey()
	err := Init(key)
	if err == nil {
		t.Errorf("expected an error when calling Init() with a key of length %d", len(key))
	}
}

func TestUsePanicsAfterBadInitialization(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
		} else {
			t.Error("no panic detected")
		}
	}()

	Init(smallKey())
	Censor(StringWithPersonalData)
}

func TestInitializedTwice(t *testing.T) {
	if err := Init(goodKey()); err != nil {
		t.Errorf("should initialize first time")
	}
	if err := Init(goodKey()); err == nil {
		t.Errorf("should error after being initialized already")
	}

}

func TestNormalUse(t *testing.T) {
	Init(goodKey())
	Censor(StringWithPersonalData)
}

func TestInitializeWithLargeKey(t *testing.T) {
	Init(bigKey())
	Censor(StringWithPersonalData)
}

func TestSameStringAlwaysCensoredToSameValue(t *testing.T) {
	Init(goodKey())
	a := Censor(StringWithPersonalData)
	b := Censor(StringWithPersonalData)
	if bytes.Equal(a, b) == false {
		t.Errorf("the same string should censor to the same value:\n%v\n%v\n", a, b)
	}
}

func TestTwoStringsAreCensoredToDifferentValues(t *testing.T) {
	Init(goodKey())
	a := Censor(StringWithPersonalData)
	b := Censor(StringWithPersonalData + " lala")
	if bytes.Equal(a, b) == true {
		t.Errorf("different strings should censor to different values:\n%v\n%v\n", a, b)
	}
}
