package keyvalue

import (
	"os"
	"testing"
)

func VerifyExpected(t *testing.T, expected map[string]string, kvs *KeyValueStore) bool {
	for k, ev := range expected {
		av, err := kvs.Get(k)
		if err != nil {
			t.Errorf("Getting key-value %s: %v", k, err)
			return true
		}
		if ev != av {
			t.Errorf("Getting key %s: Expected %s, got %s!", k, ev, av)
			return true
		}
	}

	return false
}

func VerifyUnexpected(t *testing.T, unexpected []string, kvs *KeyValueStore) bool {
	for _, key := range unexpected {
		if val, err := kvs.Get(key); err != ErrNoRows {
			t.Errorf("Getting non-existent key: Expected ErrNoRows, got (%s, %v)", val, err)
			return true
		}
	}
	return false
}

func TestKeyStore(t *testing.T) {
	fname := os.TempDir() + "/keyvalue.db"
	t.Logf("Temporary keyvalue db filename: %v", fname)

	// Remove the file first, just in case
	os.Remove(fname)

	kv, err := OpenFile(fname)
	if err != nil {
		t.Errorf("Creating session store: %v", err)
		return
	}

	if VerifyUnexpected(t, []string{"NonExistent"}, kv) {
		return
	}

	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range expected {
		if err := kv.Set(k, v); err != nil {
			t.Errorf("Setting key-value (%s, %s): %v", k, v, err)
			return
		}
	}

	if VerifyExpected(t, expected, kv) {
		return
	}

	for k, v := range expected {
		v = v + "'"
		if err := kv.Set(k, v); err != nil {
			t.Errorf("Setting key-value (%s, %s): %v", k, v, err)
			return
		}
		expected[k] = v
	}

	if VerifyExpected(t, expected, kv) {
		return
	}

	for k, v := range expected {
		nv := v + "'"
		var ov string
		if ov, err = kv.Exchange(k, nv); err != nil {
			t.Errorf("Setting key-value (%s, %s): %v", k, nv, err)
			return
		}
		if ov != v {
			t.Errorf("Unexpected old value: wanted %s, got %s!", v, ov)
			return
		}
		expected[k] = nv
	}

	// Test exchanging an empty value
	{
		k := "key4"
		v := ""
		nv := "value4"
		ov, err := kv.Exchange(k, nv)
		if err != nil {
			t.Errorf("Setting key-value (%s, %s): %v", k, nv, err)
			return
		}
		if ov != v {
			t.Errorf("Unexpected old value: wanted %s, got %s!", v, ov)
			return
		}
		expected[k] = nv
	}

	if VerifyExpected(t, expected, kv) {
		return
	}

	kv.Close()

	kv, err = OpenFile(fname)
	if err != nil {
		t.Errorf("Creating session store: %v", err)
		return
	}

	if VerifyExpected(t, expected, kv) {
		return
	}

	// Only remove the file if we were successful
	os.Remove(fname)
}
