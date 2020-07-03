package timezones

import "testing"

func TestTimezone(t *testing.T) {
	tzs, err := GetTimezoneList()
	if err != nil {
		t.Errorf("Getting timezone list: %v", err)
		return
	}

	if len(tzs) == 0 {
		t.Errorf("Empty timezone list without an error!")
		return
	}
}
