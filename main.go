package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/thamaji/devcontainer-compose/devcontainer"
	"github.com/thamaji/devcontainer-compose/parser"
	"github.com/thamaji/devcontainer-compose/spec"
	"gopkg.in/yaml.v2"
)

const DockerPath = "/usr/bin/docker"
const ComposePath = "/usr/local/bin/docker-compose"

func main() {
	spec, err := spec.GetSpec(ComposePath)
	if err != nil {
		log.Fatalln(err)
	}

	environment := devcontainer.NewEnvironment(DockerPath)
	command, err := convertArgs(os.Args[1:], spec, environment)
	if err != nil {
		log.Fatalln(err)
	}

	exitCode := command.execute(ComposePath)

	os.Exit(exitCode)
}

type command struct {
	args   []string
	onExit func()
}

func (command *command) execute(cliPath string) int {
	cmd := exec.Command(cliPath, command.args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	exitCode := 255
	if err := cmd.Start(); err == nil {
		_ = cmd.Wait()
		exitCode = cmd.ProcessState.ExitCode()
	}
	if command.onExit != nil {
		command.onExit()
	}
	return exitCode
}

func convertArgs(args []string, spec *spec.Spec, environment *devcontainer.Environment) (*command, error) {
	ctx := parser.NewContext(args)

	options, err := parser.ParseOptions(ctx, spec.GlobalOptions)
	if err != nil {
		return nil, err
	}
	var files []string
	var projectDirectory string

	newOptions := make(parser.Options, 0, len(options))

	for _, option := range options {
		switch option.Name {
		case "-f", "--file":
			files = append(files, option.Value)

		case "--project-directory":
			projectDirectory = option.Value

		default:
			newOptions.Add(option.Name, option.Value)
		}
	}

	if len(files) == 0 {
		if v, ok := os.LookupEnv("COMPOSE_FILE"); ok {
			os.Unsetenv("COMPOSE_FILE")

			sep, ok := os.LookupEnv("COMPOSE_PATH_SEPARATOR")
			if !ok {
				sep = ":"
			}

			files = strings.Split(v, sep)
		}
	}

	if len(files) == 0 {
		dir := "."
	OUTER:
		for {
			for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
				file := filepath.Join(dir, name)
				if _, err := os.Stat(file); err == nil {
					files = append(files, file)

					for _, name := range []string{"docker-compose.override.yml", "docker-compose.override.yaml", "compose.override.yml", "compose.override.yaml"} {
						file := filepath.Join(dir, name)
						if _, err := os.Stat(file); err == nil {
							files = append(files, file)
						}
					}

					break OUTER
				}
			}

			if dir == "/" {
				break
			}
			dir = filepath.Join(dir, "../")
		}
	}

	if projectDirectory == "" {
		if len(files) > 0 {
			projectDirectory = filepath.Dir(files[0])
		} else {
			projectDirectory = "."
		}
	}

	tempFiles := []string{}
	for i := range files {
		file, ok, err := createFakeComposeYaml(environment, projectDirectory, files[i])
		if err != nil {
			return nil, err
		}
		if ok {
			tempFiles = append(tempFiles, file)
		}

		newOptions.Add("--file", file)
	}

	newOptions.Add("--project-directory", projectDirectory)
	command := &command{args: newOptions.Args(), onExit: func() {
		for _, tempFile := range tempFiles {
			os.Remove(tempFile)
		}
	}}
	command.args = append(command.args, ctx.Args()...)
	return command, nil
}

func createFakeComposeYaml(environment *devcontainer.Environment, projectDirectory string, file string) (dst string, ok bool, err error) {
	defer func() {
		if recover() != nil {
			dst = file
			ok = false
		}
	}()

	text, err := ioutil.ReadFile(file)
	if err != nil {
		return file, false, nil
	}

	var data interface{}

	if err := yaml.Unmarshal(text, &data); err != nil {
		return file, false, nil
	}

	compose := reflect.ValueOf(data)
	services := compose.MapIndex(reflect.ValueOf("services")).Elem()

	for _, key := range services.MapKeys() {
		service := services.MapIndex(key).Elem()
		volumes := service.MapIndex(reflect.ValueOf("volumes")).Elem()

		for i := 0; i < volumes.Len(); i++ {
			volume := volumes.Index(i).Elem()

			switch volume.Kind() {
			case reflect.String:
				params := strings.Split(volume.String(), ":")
				if len(params) <= 0 {
					// error
					break
				}

				if t := compose.MapIndex(reflect.ValueOf("volumes")); t.IsValid() {
					if t.Elem().Kind() != reflect.Map || t.Elem().MapIndex(reflect.ValueOf(params[0])).IsValid() {
						break
					}
				}

				path := params[0]
				if !filepath.IsAbs(path) {
					path = filepath.Join(projectDirectory, path)
				}

				hostPath, err := environment.GetHostPath(path)
				if err != nil {
					return "", false, err
				}
				params[0] = hostPath

				volumes.Index(i).Set(reflect.ValueOf(strings.Join(params, ":")))

			case reflect.Map:
				vtype := volume.MapIndex(reflect.ValueOf("type")).Elem()
				if vtype.String() == "bind" {
					path := volume.MapIndex(reflect.ValueOf("source")).Elem().String()
					if !filepath.IsAbs(path) {
						path = filepath.Join(projectDirectory, path)
					}

					hostPath, err := environment.GetHostPath(path)
					if err != nil {
						return "", false, err
					}
					volume.SetMapIndex(reflect.ValueOf("source"), reflect.ValueOf(hostPath))
				}
			}
		}
	}

	f, err := os.CreateTemp(os.TempDir(), "docker-compose-*.yml")
	if err != nil {
		return "", false, err
	}
	err = yaml.NewEncoder(f).Encode(data)
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", false, err
	}

	return f.Name(), true, nil
}
