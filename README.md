Svn Repository Retrofitter
--------------------------

svn-go compromises a small golang library for working with Subversion repository dumps,
and a tool based on the library designed to clean up SVN history:

- removing unwanted properties,
- perform string replacements of file/path names,
- remove unwanted paths,
- retrofit branch changes,

Everything is configured via either command line arguments or a simple rules.yml file.

Sample invocations:

```
# Unix shell
$ go run . -read /my/repos/subversion.dump -pathinfo
```

Attempts to load the dump and then displays all the created paths.

```
# Powershell, any platform

PS> go run . -read svn.*.dump -rules rules.yml -outdir /tmp -verbose
```

Loads all the files matching "svn.`*.dump" in the current directory with verbose output,
applies any changes from rules.yml, and then recreates each file in the output path, /tmp.


```
go run . -read svn.*.dump -outfile combined.dump
```

Loads all of the input .dump files and creates a single dump file containing all of them.


## Retrofitting

This tool was primarily written to retroactively apply the structure our repository
ended up with back to the beginning of it's history.

Imagine that up until r1000 you had a single project layout:

    /Trunk
    /Branches
    /Tags

but you changed this to allow multiple projects.

    r1000 /Trunk -> /Project1/Trunk
    r1003 /Branches -> /Project1/Branches
          /Tags -> /Project1/Tags

Any path specified in the "retrofit:" list in the yml will be sought out and then actively
pushed back to where the first thing branched/copied into it was actually created.


r5:  /Trunk created
r10: /Trunk/Source/main.cpp created
r999: /Project1 created
r1000: /Trunk/Source moved to /Project1/Trunk/Source
r1010: /Trunk deleted

Running the tool with a yaml like:

```yaml
retrofit-paths:
 - Project1   # no leading slash

retrfit-props:
 - svn:ignore
 - svn:mergeinfo
```

This will move the creation of Project1 and Project1/Source back to the creation of the
original Trunk directory, and it will rewrite paths from r10 thru r1010 where the original
/Trunk was deleted, including branch references.

It will also do a similar search/replace across the svn:ignore and svn:mergeinfo
properties.

The net result is that the generated dumps will reconstruct the repository as though you
had started with /Project1/Trunk in the first place.

