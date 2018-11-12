package amiregistry

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
)

func GetAMI(region, channel string) (string, error) {

	regions, err := GetAMIData(channel)

	if err != nil {
		return "", errors.Wrapf(err, "uanble to fetch AMI for channel \"%s\": %v", channel, err)
	}

	amis, ok := regions[region]
	if !ok {
		return "", errors.Errorf("could not find region \"%s\" for channel \"%s\"", region, channel)
	}

	if ami, ok := amis["hvm"]; ok {
		return ami, nil
	}

	return "", errors.Errorf("could not find \"hvm\" image for region \"%s\" and channel \"%s\"", region, channel)
}

func GetAMIData(channel string) (map[string]map[string]string, error) {
	url := fmt.Sprintf("https://coreos.com/dist/aws/aws-%s.json", channel)
	r, err := newHttp().Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get AMI data from url \"%s\": %v", channel, err)
	}

	if r.StatusCode != 200 {
		return nil, errors.Wrapf(err, "failed to get AMI data from url \"%s\": invalid status code: %d", url, r.StatusCode)
	}

	output := map[string]map[string]string{}

	err = json.NewDecoder(r.Body).Decode(&output)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse AMI data from url \"%s\": %v", url, err)
	}
	r.Body.Close()

	return output, nil
}
