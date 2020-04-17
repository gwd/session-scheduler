# Changes required before putting up initial website 2020

- Move Interest Expression to database

- Delete schedule / timetable stuff entirely (for now)

- Modify 'Delete User' to reparent discussions to admin, rather than
  deleting.  Or maybe not: This allows an attacker to assign arbitrary
  number of things to admin.  Maybe we fix this when it becomes a
  problem.  Or maybe we delete if Admin has too many.

- Remove all references to Lars

# Short-term usability improvements

* Document `deploy.sh` and `run.sh` expected use case
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

* Config file for setting schedule rather than hard-coding
* Creating / editing schedule in webapp
* Admin: Allow admin to delete users
* Admin: Allow to delete all sessions from a user
* Make webserver actually multi-threaded (get rid of Big Lock)
* Automated backup

# Clean-up

* Lots of visual improvements
* Make structure more rational
