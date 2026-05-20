package startupmigrations

// MigrationManagerTestingKnobs contains startup migration test controls.
type MigrationManagerTestingKnobs struct {
	DisableBackfillMigrations bool
}

// ModuleTestingKnobs implements base.ModuleTestingKnobs.
func (*MigrationManagerTestingKnobs) ModuleTestingKnobs() {}
