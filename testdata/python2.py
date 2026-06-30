@decorator
def f(x):
    n = 0x1f
    b = 0b101
    o = 0o17
    fl = 1.5e3
    r = rb"raw\nbytes"
    return f"{x!r:>{n}}"
