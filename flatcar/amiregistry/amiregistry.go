package amiregistry

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

func GetAMI(region, channel string) (string, error) {

	amis, err := GetAMIData(channel)

	if err != nil {
		return "", errors.Wrapf(err, "uanble to fetch AMI for channel \"%s\": %v", channel, err)
	}

	for _, v := range amis {
		if reg, ok := v["name"]; ok {
			if reg == region {
				if hvm, ok := v["hvm"]; ok {
					return hvm, nil
				} else {
					return "", fmt.Errorf("region %s does not have a flatcar hvm ami entry", region)
				}
			}
		}
	}

	return "", errors.Errorf("could not find \"hvm\" image for region \"%s\" and channel \"%s\"", region, channel)
}

func GetAMIData(channel string) ([]map[string]string, error) {
	url := fmt.Sprintf("https://%s.release.flatcar-linux.net/amd64-usr/current/flatcar_production_ami_all.json", channel)
	r, err := newHttp().Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get AMI data from url \"%s\": %v", channel, err)
	}

	if r.StatusCode != 200 {
		return nil, errors.Wrapf(err, "failed to get AMI data from url \"%s\": invalid status code: %d", url, r.StatusCode)
	}

	output := map[string][]map[string]string{}

	err = json.NewDecoder(r.Body).Decode(&output)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse AMI data from url \"%s\": %v", url, err)
	}
	r.Body.Close()

	return output["amis"], nil
}
