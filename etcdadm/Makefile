.PHONY: test-integration
test-integration:
	if ! [ -f Vagrantfile ]; then curl -o Vagrantfile https://raw.githubusercontent.com/coreos/coreos-vagrant/master/Vagrantfile; fi
	sed -i original 's/$update_channel = "alpha"/$update_channel = "stable"/' Vagrantfile
	if vagrant status | grep core-01 | grep running; then vagrant destroy -f; fi
	vagrant up
	vagrant ssh -c 'rm -f etcdadm test test.env'
	vagrant ssh-config > ssh-config
	scp -F ssh-config etcdadm test test.env core@core-01:
	vagrant ssh -c 'sudo ./test integration'
