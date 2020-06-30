package event

import (
	"math/rand"
	"testing"

	"github.com/icrowley/fake"
)

func testNewLocation(t *testing.T) (Location, bool) {
	loc := Location{
		LocationName: fake.Word(),
		LocationURL:  "https://" + fake.DomainName(),
		IsPlace:      rand.Intn(2) == 0,
		Capacity:     rand.Intn(200) + 1,
	}

	if _, err := NewLocation(&loc); err != nil {
		t.Errorf("ERROR: Creating new location: %v", err)
		return loc, true
	}

	return loc, false
}

func compareLocations(l1, l2 *Location, t *testing.T) bool {
	ret := true
	if l1.LocationName != l2.LocationName {
		t.Logf("mismatch LocationName: %v != %v",
			l1.LocationName, l2.LocationName)
		ret = false
	}
	if l1.LocationURL != l2.LocationURL {
		t.Logf("mismatch LocationURL: %v != %v",
			l1.LocationURL, l2.LocationURL)
		ret = false
	}
	if l1.IsPlace != l2.IsPlace {
		t.Logf("mismatch IsPlace: %v != %v",
			l1.IsPlace, l2.IsPlace)
		ret = false
	}
	if l1.Capacity != l2.Capacity {
		t.Logf("mismatch Capacity: %v != %v",
			l1.Capacity, l2.Capacity)
		ret = false
	}
	return ret
}

func testUnitLocation(t *testing.T) (exit bool) {
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	t.Logf("Trying to make invalid locations")
	{
		_, err := NewLocation(&Location{LocationName: "", LocationURL: "<URL>", Capacity: 10})
		if err == nil {
			t.Errorf("ERROR: Created location with empty name!")
			return
		}

		_, err = NewLocation(&Location{LocationName: "Blah", LocationURL: "<URL>", Capacity: 0})
		if err == nil {
			t.Errorf("ERROR: Created location with zero capacity!")
			return
		}

		_, err = NewLocation(&Location{LocationName: "Blah", LocationURL: "<URL>", Capacity: -100})
		if err == nil {
			t.Errorf("ERROR: Created location with negative capacity!")
			return
		}
	}

	// Make 6 locations for testing
	testLocationCount := 6
	locations := make([]Location, testLocationCount)

	t.Logf("Making some sample locations")
	for i := 0; i < len(locations); i++ {
		subexit := false
		locations[i], subexit = testNewLocation(t)
		if subexit {
			return
		}

		if locations[i].LocationID != LocationID(i+1) {
			t.Errorf("ERROR Unexpected LocationID: expected %v, got %v!",
				LocationID(i+1), locations[i].LocationID)
			return
		}

		// Look for that location by did
		gotloc, err := LocationFindById(locations[i].LocationID)
		if err != nil {
			t.Errorf("Finding the location we just created by ID: %v", err)
			return
		}
		if gotloc == nil {
			t.Errorf("Couldn't find just-created location by id %v!", locations[i].LocationID)
			return
		}
		if !compareLocations(&locations[i], gotloc, t) {
			t.Errorf("Location data mismatch")
			return
		}
	}

	t.Logf("Testing Corner cases")
	{
		// Try to find a non-existent ID.  Should return nil for both.
		gotdisc, err := LocationFindById(LocationID(len(locations) + 1))
		if err != nil {
			t.Errorf("Unexpected error finding non-existent location: %v", err)
			return
		}
		if gotdisc != nil {
			t.Errorf("Unexpectedly got non-existent location!")
			return
		}
	}

	t.Logf("Testing LocationUpdate")
	for i := range locations {
		copy := locations[i]
		copy.LocationName = fake.Word()
		copy.LocationURL = "https://" + fake.DomainName()
		err := LocationUpdate(&copy)
		if err != nil {
			t.Errorf("Updating location: %v", err)
			return
		}

		gotloc, err := LocationFindById(locations[i].LocationID)
		if err != nil {
			t.Errorf("Finding the location we just created by ID: %v", err)
			return
		}
		if gotloc == nil {
			t.Errorf("Couldn't find location by id %v!", locations[i].LocationID)
			return
		}
		if !compareLocations(&copy, gotloc, t) {
			t.Errorf("Location data mismatch")
			return
		}
		locations[i] = *gotloc
	}

	t.Logf("Testing LocationGetAll")
	{
		gotlocs, err := LocationGetAll()
		if err != nil {
			t.Errorf("Getting locations: %v", err)
			return
		}
		if len(gotlocs) != len(locations) {
			t.Errorf("Unexpected number from gotlocs: expected %v, got %v",
				len(locations), len(gotlocs))
			return
		}

		for i := range locations {
			if !compareLocations(&locations[i], &gotlocs[i], t) {
				t.Errorf("Location data mismatch!")
				return
			}
		}
	}

	t.Logf("Testing DeleteLocation")
	for i := range locations {
		err := DeleteLocation(locations[i].LocationID)
		if err != nil {
			t.Errorf("Deleting location: %v", err)
			return
		}

		// Delete it again, should get ErrorLocationNotFound
		err = DeleteLocation(locations[i].LocationID)
		if err != ErrLocationNotFound {
			t.Errorf("Unexpected err from second delete: %v", err)
			return
		}

		// Try to find it.  Should return nil for both.
		gotdisc, err := LocationFindById(locations[i].LocationID)
		if err != nil {
			t.Errorf("Unexpected error finding deleted location: %v", err)
			return
		}
		if gotdisc != nil {
			t.Errorf("Unexpectedly got deleted location!")
			return
		}
	}

	return false
}
