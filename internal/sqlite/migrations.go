package sqlite

var systemPatches = PatchSet{
	StatementPatch("gribble/init", "base-system", 1,
		`CREATE TABLE runners (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	token TEXT UNIQUE ON CONFLICT ABORT,
	description TEXT,
	run_untagged BOOLEAN,
	locked BOOLEAN,
	active BOOLEAN,
	timeout INT
)`,
		`CREATE TABLE jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT
)`,
	),
}
