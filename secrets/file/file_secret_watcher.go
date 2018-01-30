package file

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"

	filewatch "github.com/solo-io/glue/adapters/file/watcher"
	"github.com/solo-io/glue/pkg/log"
	"github.com/solo-io/glue/secrets"
)

// FileWatcher uses .yml files in a directory
// to watch secrets
type fileWatcher struct {
	file           string
	secretsToWatch []string
	secrets        chan secrets.SecretMap
	errors         chan error
}

func NewSecretWatcher(file string, syncFrequency time.Duration) (*fileWatcher, error) {
	secrets := make(chan secrets.SecretMap)
	errors := make(chan error)
	fw := &fileWatcher{
		secrets: secrets,
		errors:  errors,
		file:    file,
	}
	if err := filewatch.WatchFile(file, func(_ string) {
		fw.updateSecrets()
	}, syncFrequency); err != nil {
		return nil, fmt.Errorf("failed to start filewatcher: %v", err)
	}

	return fw, nil
}

func (fw *fileWatcher) updateSecrets() {
	secretMap, err := fw.getSecrets()
	if err != nil {
		fw.errors <- err
		return
	}
	// ignore empty configs / no secrets to watch
	if len(secretMap) == 0 {
		return
	}
	fw.secrets <- secretMap
}

// triggers an update
func (fw *fileWatcher) TrackSecrets(secretRefs []string) {
	fw.secretsToWatch = secretRefs
	fw.updateSecrets()
}

func (fw *fileWatcher) Secrets() <-chan secrets.SecretMap {
	return fw.secrets
}

func (fw *fileWatcher) Error() <-chan error {
	return fw.errors
}

func (fw *fileWatcher) getSecrets() (secrets.SecretMap, error) {
	yml, err := ioutil.ReadFile(fw.file)
	if err != nil {
		return nil, err
	}
	var secretMap secrets.SecretMap
	err = yaml.Unmarshal(yml, &secretMap)
	if err != nil {
		return nil, err
	}
	desiredSecrets := make(secrets.SecretMap)
	for _, ref := range fw.secretsToWatch {
		data, ok := secretMap[ref]
		if !ok {
			log.Printf("ref %v not found", ref)
			return nil, fmt.Errorf("secret ref %v not found in file %v", ref, fw.file)
		}
		log.Printf("ref found: %v", ref)
		desiredSecrets[ref] = data
	}

	return desiredSecrets, err
}