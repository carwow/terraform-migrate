package main

import (
	"os"
	"testing"
)

func TestGetCurrentVersionIsUnset(t *testing.T) {
	err := os.Unsetenv("TF_MIGRATION_VERSION")
	if err != nil {
		t.Fatal(err)
	}
	version, err := getCurrentVersion()
	if err != nil {
		t.Error(err)
	}
	if version != 0 {
		t.Error(version)
	}
}

func TestGetCurrentVersionIsSet(t *testing.T) {
	err := os.Setenv("TF_MIGRATION_VERSION", "99")
	if err != nil {
		t.Fatal(err)
	}
	version, err := getCurrentVersion()
	if err != nil {
		t.Error(err)
	}
	if version != 99 {
		t.Error(version)
	}
}

func TestGetCurrentVersionIsNotNumber(t *testing.T) {
	err := os.Setenv("TF_MIGRATION_VERSION", "boom")
	if err != nil {
		t.Fatal(err)
	}
	version, err := getCurrentVersion()
	if err.Error() != "failed to parse current version: strconv.Atoi: parsing \"boom\": invalid syntax" {
		t.Error(err)
	}
	if version != -1 {
		t.Error(version)
	}
}

func TestFindMigrationFileWithNoFile(t *testing.T) {
	file, err := findMigrationFile("migrate_test", 1)
	if err != nil {
		t.Error(err)
	}
	if file != "" {
		t.Error(file)
	}
}

func TestFindMigrationFileWithFiles(t *testing.T) {
	err := os.Mkdir("migrate_test", 0777)
	if err != nil {
		t.Fatal(err)
	}
	os.Create("migrate_test/01_a.sh")
	os.Create("migrate_test/2_b.sh")
	os.Create("migrate_test/2_c.sh")
	os.Create("migrate_test/3_d.sh")

	file, err := findMigrationFile("migrate_test", 1)
	if err != nil {
		t.Error(err)
	}
	if file != "" {
		t.Error(file)
	}

	file, err = findMigrationFile("migrate_test", 2)
	if err.Error() != "2 files found with version 2" {
		t.Error(err)
	}
	if file != "" {
		t.Error(file)
	}

	file, err = findMigrationFile("migrate_test", 3)
	if err != nil {
		t.Error(err)
	}
	if file != "migrate_test/3_d.sh" {
		t.Error(file)
	}

	err = os.RemoveAll("migrate_test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenameBackendFileMissingFile(t *testing.T) {
	err := renameBackendFile()
	if err.Error() != "failed to disable backend: rename backend.tf backend.tf.disable: no such file or directory" {
		t.Error(err)
	}
}

func TestRenameBackendFile(t *testing.T) {
	_, err := os.Create("backend.tf")
	if err != nil {
		t.Fatal(err)
	}
	err = renameBackendFile()
	if err != nil {
		t.Error(err)
	}
	err = os.Remove("backend.tf.disable")
	if err != nil {
		t.Error(err)
	}
}
