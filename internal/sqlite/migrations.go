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
			max_timeout INT
		)`,
		`CREATE TABLE tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tag TEXT UNIQUE ON CONFLICT ABORT
		)`,
		`CREATE TABLE runner_tags (
			tag INTEGER,
			runner INTEGER,
			PRIMARY KEY (tag, runner)
		)`,
		`CREATE TABLE jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT
		)`,
	),
}
