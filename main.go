package main

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}

	command := os.Args[1]
	var err error

	switch command {
	case "help":
		help()
	case "init":
		err = initCmd()
	case "local":
		err = local(os.Args)
	case "apply":
		err = apply()
	case "force-unlock":
		err = unlock()
	default:
		help()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "command failed: %s\n", err)
		os.Exit(1)
	}
}

func help() {
	fmt.Printf(`%s [command]

Commands:
	init			sets up lock in CircleCI
				requires CIRCLE_TOKEN environment variable

	local [VERSION]		runs next migration file against a local copy
				of the terraform state
				requires backend.tf file to include terraform
				backend config

	apply			runs next migration file

	force-unlock		releases lock in CircleCI
				requires CIRCLE_TOKEN environment variable

	help			displays this text
`, path.Base(os.Args[0]))
}

func initCmd() error {
	lockKey := lockKey()
	lock := os.Getenv(lockKey)
	if lock != "" {
		return nil
	}
	return unlock()
}

const migrationsDir = "migrations"

func local(args []string) error {
	var nextVersion int
	var err error

	switch len(args) {
	case 2:
		nextVersion, err = getNextVersion()
	case 3:
		nextVersion, err = strconv.Atoi(args[2])
	default:
		err = fmt.Errorf("Too many arguments!")
	}
	if err != nil {
		return err
	}

	migration, err := findNextMigration(nextVersion, migrationsDir)
	if err != nil {
		return err
	}
	if migration == nil {
		fmt.Println("Already up to date.")
		return nil
	}

	err = disableTerraformBackendOrSkip()
	if err != nil {
		return err
	}

	err = runMigrationFile(migration.file)
	if err != nil {
		return err
	}

	return nil
}

func apply() error {
	nextVersion, err := getNextVersion()
	if err != nil {
		return err
	}
	migration, err := findNextMigration(nextVersion, migrationsDir)
	if err != nil {
		return err
	}
	if migration == nil {
		fmt.Println("Already up to date.")
		return nil
	}

	err = lock()
	if err != nil {
		return err
	}

	err = setMigrationVersion(migration.version)
	if err != nil {
		return err
	}

	err = runMigrationFile(migration.file)
	if err != nil {
		return err
	}

	err = unlock()
	if err != nil {
		return err
	}

	return nil
}

type migration struct {
	version int
	file    string
}

func getNextVersion() (int, error) {
	currentVersion, err := getCurrentVersion()
	if err != nil {
		return 0, err
	}

	return currentVersion + 1, nil
}

func findNextMigration(nextVersion int, migrationsDir string) (*migration, error) {
	file, err := findMigrationFile(migrationsDir, nextVersion)
	if err != nil {
		return nil, err
	}
	if file == "" {
		return nil, fmt.Errorf("No migration file")
	}

	return &migration{
		version: nextVersion,
		file:    file,
	}, nil
}

const migrationVersionEnv = "TF_MIGRATION_VERSION"

func getCurrentVersion() (int, error) {
	envVersion := os.Getenv(migrationVersionEnv)
	if envVersion == "" {
		return 0, nil
	}

	version, err := strconv.Atoi(envVersion)
	if err != nil {
		return -1, fmt.Errorf("failed to parse current version: %s", err)
	}

	return version, nil
}

func findMigrationFile(migrationsDir string, version int) (string, error) {
	files, err := filepath.Glob(fmt.Sprintf(migrationsDir+"/%d_*.sh", version))
	if err != nil {
		return "", fmt.Errorf("failed to find migration file: %s", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no migration file found")
	}
	if len(files) > 1 {
		return "", fmt.Errorf("%d files found with version %d", len(files), version)
	}

	return files[0], nil
}

func disableTerraformBackendOrSkip() error {
	_, err := os.Stat("backend.tf.disable")
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return disableTerraformBackend()
	}

	return err
}

func disableTerraformBackend() error {
	err := renameBackendFile()
	if err != nil {
		return err
	}

	err = terraformInit()
	if err != nil {
		return err
	}

	return nil
}

func renameBackendFile() error {
	err := os.Rename("backend.tf", "backend.tf.disable")
	if err != nil {
		return fmt.Errorf("failed to disable backend: %s", err)
	}

	return nil
}

func terraformInit() error {
	initCmd := exec.Command("terraform", "init", "-force-copy", "-lock-timeout=60s")
	output, err := initCmd.CombinedOutput()
	fmt.Printf("%s", output)
	if err != nil {
		return fmt.Errorf("failed to terraform init: %s", err)
	}

	return nil
}

func runMigrationFile(file string) error {
	migrationCmd := exec.Command(file)
	output, err := migrationCmd.CombinedOutput()
	fmt.Printf("%s", output)
	if err != nil {
		return fmt.Errorf("failed to run migration: %s", err)
	}

	return nil
}

func setMigrationVersion(version int) error {
	err := circleSetenv(migrationVersionEnv, strconv.Itoa(version))
	if err != nil {
		return fmt.Errorf("failed to set version: %s", err)
	}

	return nil
}

const tfEnv = "TF_ENV"
const migrationLockEnv = "TF_MIGRATION_LOCK"

func lockKey() string {
	env := os.Getenv(tfEnv)
	if env == "" {
		return migrationLockEnv
	}
	return migrationLockEnv + "_" + env
}

func lock() error {
	lockKey := lockKey()
	lock := os.Getenv(lockKey)
	if lock == "" {
		return fmt.Errorf("lock not found")
	}

	token, err := circleToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/%s_%s%s", circleEnvvarUrl, lockKey, lock, token)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		return fmt.Errorf("lock not found")
	}
	if res.StatusCode != 200 {
		resBody, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("failed to lock: %s\n%s", res.Status, resBody)
	}

	return nil
}

func unlock() error {
	lock, err := generateUUID()
	if err != nil {
		return fmt.Errorf("failed to unlock: %s", err)
	}

	lockKey := lockKey()
	err = circleSetenv(lockKey+"_"+lock, lock)
	if err != nil {
		return fmt.Errorf("failed to unlock: %s", err)
	}

	err = circleSetenv(lockKey, lock)
	if err != nil {
		return fmt.Errorf("failed to unlock: %s", err)
	}

	return nil
}

func generateUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %v", err)
	}

	return fmt.Sprintf("%x_%x_%x_%x_%x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16]), nil
}

const circleUrl = "https://circleci.com/api/v1.1/project"
const circleProjectUrl = circleUrl + "/github/carwow/carwow-terraform"
const circleEnvvarUrl = circleProjectUrl + "/envvar"

func circleToken() (string, error) {
	token := os.Getenv("CIRCLE_TOKEN")
	if token == "" {
		return "", fmt.Errorf("missing CIRCLE_TOKEN env var")
	}
	return "?circle-token=" + token, nil
}

func circleSetenv(name string, value string) error {
	token, err := circleToken()
	if err != nil {
		return err
	}

	body := strings.NewReader(fmt.Sprintf(`{"name":"%s","value":"%s"}`, name, value))
	res, err := http.Post(circleEnvvarUrl+token, "application/json", body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		resBody, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("failed to set env in CircleCI: %s\n%s", res.Status, resBody)
	}

	return nil
}
