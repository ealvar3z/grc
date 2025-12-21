# grc usage guide

This guide assumes familiarity with the Plan 9 rc shell.

---

## Variables are lists

```rc
x=hello
y=(a b c)
echo $x
echo $y
```

Free carets and concatenation

```rc
x=foo
echo $x.c        # expands as $x^.c
echo $x^bar
```

Concatenation rules:

- pairwise if lengths match
- distributive if one side has length 1
- error otherwise

Assignments

Standalone:

```rc
x=world
echo $x
```

Prefix (scoped):

```rc
x=parent
x=child echo $x
echo $x
```

Functions

```rc
fn greet {
    echo hello $1
}

greet world
```

Dynamic scoping applies.

Command substitution

```rc
echo `{ echo a b }
```

Backquotes produce lists split on whitespace.

Globbing

```rc
echo *.go
```

If a glob matches nothing, it remains literal.

Background jobs

```rc
sleep 5 &
jobs
```

Background PIDs are tracked in $apid.

Prompt

Set the prompt using a list:

```rc
prompt=(β grc)
```

Debugging

Print the execution plan:

```sh
grc -p
```

Trace executed commands:

```sh
grc -x
```

Parse only:

```sh
grc -n
```
