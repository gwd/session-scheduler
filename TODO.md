# Changes before final websete

- Timetable slots cannot be removed if they contain an assigned
session

- Scheduled sessions cannot be deleted (and no error is reported!)

- A way to set your own default timezone 

- Scheduler state (current / whatever)

- Add back random search

- Modify 'Delete User' to reparent discussions to admin, rather than
  deleting.  Or maybe not: This allows an attacker to assign arbitrary
  number of things to admin.  Maybe we fix this when it becomes a
  problem.  Or maybe we delete if Admin has too many.

# Short-term usability improvements

* Document `deploy.sh` and `run.sh` expected use case
* Add editTimetable to command-line help
* Document editTimetable
* Document inactive mode, &c

# Priority improvements

* Deal with account creation restriction
 - Have session 'moderation' for 'unverified' accounts
* Having a place to take notes during the session
    * Integration of etherpad-like functionality?
    * Automatically creating an etherpad (with a link)?
  	    * `etherpad.net/p/$UUID`
	    * `UUID` could be the session uuid, or a new one (since session uuids are pseudo-public)
    * hackmd.io?
* Avoid scheduling multiple sessions proposed by the same person at the same time

# Potential improvements

* Creating / editing schedule in webapp
* Automated backup

# Clean-up

* Lots of visual improvements
* Make code structure more rational

* Use "embed" to package up the templates &c
