package migration

// InventoryItem is the immutable file-backed contract consumed by migration
// smoke tests. It intentionally comes from DefaultMigrations so discovery,
// ordering, migration kind, and checksums cannot drift into a second registry.
type InventoryItem struct {
	Version     string
	Checksum    string
	Kind        MigrationKind
	Description string
}

// DefaultInventory resolves the complete default migration registry without a
// database connection. Checksums use the same canonical implementation as the
// runtime runner.
func DefaultInventory(projectRoot string) ([]InventoryItem, error) {
	migrations := DefaultMigrations(projectRoot)
	if err := validateMigrations(migrations); err != nil {
		return nil, err
	}
	items := make([]InventoryItem, 0, len(migrations))
	for _, item := range migrations {
		_, checksum, err := migrationBodyAndChecksum(item)
		if err != nil {
			return nil, err
		}
		items = append(items, InventoryItem{
			Version:     item.Version,
			Checksum:    checksum,
			Kind:        item.Kind,
			Description: item.Description,
		})
	}
	return items, nil
}
