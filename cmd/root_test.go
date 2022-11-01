package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acouvreur/sablier/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestPrecedence(t *testing.T) {
	// Run the tests in a temporary directory
	tmpDir := os.TempDir()
	testDir, err := os.Getwd()
	require.NoError(t, err, "error getting the current working directory")
	defer os.Chdir(testDir)
	err = os.Chdir(tmpDir)
	require.NoError(t, err, "error changing to the temporary test directory")

	// CHANGE `startCmd` behavior to only print the config, this is for testing purposes only
	newStartCommand = mockStartCommand

	t.Run("config file", func(t *testing.T) {
		configB, err := os.ReadFile(filepath.Join(testDir, "testdata", "config.yml"))
		require.NoError(t, err, "error reading test config file")
		err = ioutil.WriteFile(filepath.Join(tmpDir, "config.yml"), configB, 0644)
		require.NoError(t, err, "error writing test config file")
		defer os.Remove(filepath.Join(tmpDir, "config.yml"))

		wantConfig, err := ioutil.ReadFile(filepath.Join(testDir, "testdata", "config_yaml_wanted.json"))
		require.NoError(t, err, "error reading test config file")

		conf = config.NewConfig()
		cmd := NewRootCommand()
		output := &bytes.Buffer{}
		cmd.SetOut(output)
		cmd.SetArgs([]string{"start"})
		cmd.Execute()

		gotOutput := output.String()

		assert.Equal(t, string(wantConfig), gotOutput)
	})

	t.Run("env var", func(t *testing.T) {
		// 1. Load Config file for precedence assertions
		configB, err := os.ReadFile(filepath.Join(testDir, "testdata", "config.yml"))
		require.NoError(t, err, "error reading test config file")
		err = ioutil.WriteFile(filepath.Join(tmpDir, "config.yml"), configB, 0644)
		require.NoError(t, err, "error writing test config file")
		defer os.Remove(filepath.Join(tmpDir, "config.yml"))

		setEnvsFromFile(filepath.Join(testDir, "testdata", "config.env"))
		defer unsetEnvsFromFile(filepath.Join(testDir, "testdata", "config.env"))

		wantConfig, err := ioutil.ReadFile(filepath.Join(testDir, "testdata", "config_env_wanted.json"))
		require.NoError(t, err, "error reading test config file")

		conf = config.NewConfig()
		cmd := NewRootCommand()
		output := &bytes.Buffer{}
		cmd.SetOut(output)
		cmd.SetArgs([]string{"start"})
		cmd.Execute()

		gotOutput := output.String()

		assert.Equal(t, string(wantConfig), gotOutput)
	})

	t.Run("flag", func(t *testing.T) {
		// 1. Load Config file for precedence assertions
		configB, err := os.ReadFile(filepath.Join(testDir, "testdata", "config.yml"))
		require.NoError(t, err, "error reading test config file")
		err = ioutil.WriteFile(filepath.Join(tmpDir, "config.yml"), configB, 0644)
		require.NoError(t, err, "error writing test config file")
		defer os.Remove(filepath.Join(tmpDir, "config.yml"))

		// 2. Load envs variable for precedence assertions
		setEnvsFromFile(filepath.Join(testDir, "testdata", "config.env"))
		defer unsetEnvsFromFile(filepath.Join(testDir, "testdata", "config.env"))

		wantConfig, err := ioutil.ReadFile(filepath.Join(testDir, "testdata", "config_cli_wanted.json"))
		require.NoError(t, err, "error reading test config file")

		cmd := NewRootCommand()
		output := &bytes.Buffer{}
		conf = config.NewConfig()
		cmd.SetOut(output)
		cmd.SetArgs([]string{
			"start",
			"--provider.name", "cli",
			"--server.port", "3333",
			"--server.base-path", "/cli/",
			"--storage.file", "/tmp/cli.json",
			"--sessions.default-duration", "3h",
			"--sessions.expiration-interval", "3h",
			"--logging.level", "info",
			"--strategy.dynamic.custom-themes-path", "/tmp/cli/themes",
			"--strategy.dynamic.default-theme", "cli",
			"--strategy.dynamic.default-refresh-frequency", "3h",
			"--strategy.blocking.default-timeout", "3h",
		})
		cmd.Execute()

		gotOutput := output.String()

		assert.Equal(t, string(wantConfig), gotOutput)
	})
}

func setEnvsFromFile(path string) {
	readFile, err := os.Open(path)

	if err != nil {
		panic(err)
	}

	defer readFile.Close()

	if err != nil {
		panic(err)
	}

	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		splitted := strings.Split(fileScanner.Text(), "=")
		os.Setenv(splitted[0], splitted[1])
	}
}

func unsetEnvsFromFile(path string) {
	readFile, err := os.Open(path)

	if err != nil {
		panic(err)
	}

	defer readFile.Close()

	if err != nil {
		panic(err)
	}

	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		splitted := strings.Split(fileScanner.Text(), "=")
		os.Unsetenv(splitted[0])
	}
}

func mockStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Sablier server",
		Run: func(cmd *cobra.Command, args []string) {
			viper.Unmarshal(&conf)

			out := cmd.OutOrStdout()

			encoder := json.NewEncoder(out)

			encoder.SetIndent("", "  ")
			encoder.Encode(conf)
		},
	}
	return cmd
}
