# gasshuku-isucon-20230909

## usage

### initialize

#### ~create a new repository~ already done

Create a new repository from https://github.com/bgpat/isucon-boilerplate/generate.

#### add members

Invite your team members from https://github.com/dbgofy/gasshuku-isucon-20230909/settings/access.
After them have confirmed, run [Update SSH Keys](https://github.com/dbgofy/gasshuku-isucon-20230909/actions?query=workflow%3A%22Update+SSH+Keys%22).

#### generate and register ssh deploy key

```bash
ssh-keygen -t rsa
cat ~/.ssh/id_rsa.pub
```

Copy the result and register as a [deploy key](https://github.com/bgpat/isucon-boilerplate/settings/keys/new).

#### install to the competition server

Run following commands in the each competition server:

```bash
cd /
git init
git remote add origin git@github.com:dbgofy/gasshuku-isucon-20230909.git
git fetch origin master
git reset --hard FETCH_HEAD
make
```

Add deploy key and run `git push -u origin master`.

### run monitoring servers

```bash
make monitoring
```

Access to 3999 port from your browser.

### edit config file

```bash
vim /etc/nginx/nginx.conf
```

### track source files by git

```bash
git add -f *.go
```
