case "$1" in
	a) echo first ;;
	b) echo second ;;
esac
x=$(( 1 + 2 ))
y=${VAR:-default}
z=$(ls -la)
w=`pwd`
s=$'ansi\nstring'
