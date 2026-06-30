#!/bin/bash
# greet the user
name="world"
echo "hello $name"
for i in 1 2 3; do
	echo $i
done
if [ -f foo ]; then
	cat foo
fi
