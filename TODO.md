# Changes before final websete

- Make "break" a  more obviously different color

- Highlight schedule according to interest, to make it easier to find
  out what your own schedule is

- Use "embed" to package up the templates &c
 XXX debian-testing is in freeze, stuck on golang 1.15; embed is only
 available in 1.16.

- Timetable slots cannot be removed if they contain an assigned
session

- Locations (probably) cannot be removed if something has been
scheduled in them

- If locations change type or capacity, schedule should be nullified

- Scheduler state (current / whatever)

- Add back random search

- Modify 'Delete User' to reparent discussions to admin, rather than
  deleting.  Or maybe not: This allows an attacker to assign arbitrary
  number of things to admin.  Maybe we fix this when it becomes a
  problem.  Or maybe we delete if Admin has too many.

# Short-term usability improvements

* Document `deploy.sh` expected use case
* Add editTimetable to command-line help
* Document editTimetable
* Document inactive mode, &c

# Priority improvements

* Having a place to take notes during the session
    * Integration of etherpad-like functionality?
    * Automatically creating an etherpad (with a link)?
  	    * `etherpad.net/p/$UUID`
	    * `UUID` could be the session uuid, or a new one (since session uuids are pseudo-public)
    * hackmd.io?
* Avoid scheduling multiple sessions proposed by the same person at the same time

# Potential improvements

* Automated backup

* Creating / editing timetable in webapp

# Clean-up

* Lots of visual improvements
* Make code structure more rational

* Use mirrorData in `heuristic_test.go`, `interest_test.go`,
  `possibleslot_test.go`, and `transaction_test.go`

* Use testNewUsers in `heuristic_test.go`, `interest_test.go`,
  `possibleslot_test.go`, and `transaction_test.go`
