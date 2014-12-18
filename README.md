GoListen
========

This project is a naive port of one ruby gem ``listen``.

The aim of this project is to solve an issue about file change detections inside a virtual machine, such Virtual Box
handled by a Vagrant Box.

If you are using a solution like ``gulp`` or ``grunt``, they both provide a solution to watch files for change and start
tasks. However, the watch feature doesn't work well inside a virtual machine:
 - shared folders are slow, so the change detection can take a while depends on the project size (up to 10s).
 - shared folders does not allow to use native FS features to detect file change, so the polling method consumes a lot
  of resources.

As always, solutions exist: the present one, is to watch files on the host machine and send signals to an internal
process inside the guest machine to refresh the code.

golisten installation
---------------------

  1. Retrieve the valid version for your OS (only OSX has been tested) on https://github.com/ekino/golisten/releases
  2. You need to retrieve the ip address of the bridge (something like: 192.168.30.1)
  3. Start the process ``golisten`` with the option ``-server=192.168.30.1:4001`` flag and the ``-server-format=gem-listen`` flag.
  A typical command will be: ``./golisten -path ~/myproject/with/ -server-format="gem-listen" -server="192.168.30.1:4000"``

gulp integration
----------------

 - Make sure you have ``gulp-util`` installed
 - Install ``watch-network`` module version >=0.1.7
 - Add the ``watch-network`` task into your ``gulpfile.js``
 - Run the command ``gulp watch-network`` (the golisten server must run on the host machine)

    ```js
    gulp.task('watch-network', function() {
      var watch = WatchNetwork({
        gulp: gulp,
        host: '192.168.30.1',
        port: '4000',
        configs: [{
          tasks: 'build',
          onLoad: true
        }, {
          patterns: ['scripts/*.jsx', 'scripts/*/*.jsx'],
          tasks: 'browserify'
        }, {
          patterns: 'styles/*.scss',
          tasks: 'styles'
        }]
      })

      watch.initialize();
    });
    ```

If you are using ``golang``, you can also get the binary by running the command ``go get github.com/ekino/golisten``.

all in one
----------

It is possible to start the watcher command and the remote command in one command:

    ./golisten \
     -path ~/Projects/go/src/github.com/rande/gonodeexplorer \
     -server="192.168.30.1:4000"  \
     -server-format="gem-listen"  \
     -parallel-command="vagrant ssh -c \"cd /vagrant/go/src/github.com/rande/gonodeexplorer && gulp watch-network\"" \
     -verbose

Let's explain the options:

  - ``path``: specify the path to listen
  - ``server``: start a TCP server for remote process to listen to local change
  - ``server-format``: send the format using the ruby gem format
  - ``parallel-command``: start a command, the command will be restarted if the process exit.
  - ``verbose``: very verbose output

config file
-----------

You can export a configuration into a toml format.

    ./golisten -p > config.toml

And use this configuration file:

    ./golisten -c config.toml



