package sqlite

var systemPatches = PatchSet{
	// Initialize gribble tables and indices
	StatementPatch("gribble/init", "base-system", 1,
		`CREATE TABLE runners(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE ON CONFLICT ABORT,
			description TEXT,
			run_untagged BOOLEAN DEFAULT 0,
			locked BOOLEAN DEFAULT 0,
			active BOOLEAN DEFAULT 0,
			max_timeout INTEGER DEFAULT 0,
			deleted BOOLEAN DEFAULT 0
		)`,

		`CREATE TABLE tags(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tag TEXT UNIQUE ON CONFLICT ABORT
		)`,

		`CREATE TABLE runner_tags(
			tag INTEGER,
			runner INTEGER,

			PRIMARY KEY(tag, runner),
			FOREIGN KEY(tag) REFERENCES tags(id),
			FOREIGN KEY(runner) REFERENCES runners(id)
		)`,
		`CREATE INDEX runner_tags_by_runner ON runner_tags (runner)`,

		`CREATE TABLE jobs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			runner INTEGER,
			state TEXT DEFAULT 'pending', -- gciwire.JobState
			spec JSON, -- common.JobSpec

			FOREIGN KEY(runner) REFERENCES runners(id)
		)`,

		`CREATE TABLE job_depends(
			-- dest is dependent on src
			dest INTEGER,
			src INTEGER CHECK(src < dest),
			fetch_artifacts BOOLEAN DEFAULT 0,

			PRIMARY KEY(src, dest),
			FOREIGN KEY(src) REFERENCES jobs(id),
			FOREIGN KEY(dest) REFERENCES jobs(id)
		)`,
	),
}
