package main

import (
	"fmt"
	"os"

	"github.com/davidlazar/go-crypto/secretkey"
)

func Keygen(keyPath string) error {
	var newKey bool

	key, err := secretkey.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			newKey = true
			key = secretkey.New()
		} else {
			return fmt.Errorf("secretkey.ReadFile: %s", err)
		}
	}

	if newKey {
		fmt.Fprintf(os.Stderr, "\nCreating new key file: %s\n", keyPath)
	} else {
		fmt.Fprintf(os.Stderr, "\nUpdating passphrase for key file: %s\n", keyPath)
	}

	if err = secretkey.WriteFile(key, keyPath); err != nil {
		return fmt.Errorf("secretkey.WriteFile: %s", err)
	}

	if newKey {
		fmt.Fprintf(os.Stderr, "\nKey file created successfully: %s\n", keyPath)
	} else {
		fmt.Fprintf(os.Stderr, "\nPassphrase updated successfully: %s\n", keyPath)
	}

	fmt.Fprintf(os.Stderr, "You should now write down your key file and store it somewhere safe!\n")
	return nil
}
