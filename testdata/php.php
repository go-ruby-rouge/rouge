namespace App\Models;

use App\Contracts\Repository;
use App\Helpers\format_date;
use function App\util\to_string;

const MAX_ITEMS = 100;

#[Route("/users")]
class User extends Model implements Repository
{
    public int $id = 0;
    private string $name = "guest";
    protected ?array $roles = ['admin', 'user'];
    public static $shared;

    public function __construct(int $id, string $name = "anon", ...$rest)
    {
        // set up fields
        $this->id = $id;
        $this->name = $name;
        $total = 0x1F + 0b1010 + 0o17 + 3.14 + 42_000 + .5e3;
        $greeting = "Hi, {$name}! id=${id} raw=$id\n";
        $raw = 'single \' quote';
        $cmd = `ls -la`;
        $doc = <<<EOT
      hello $name
      EOT;
        echo $greeting, MAX_ITEMS;
        return new self($id);
    }

    /** doc comment */
    public function getName(): string
    {
        # hash comment
        return $this->name ?? "unknown";
    }

    public function value(): int use ($id) {
        return $id;
    }
}

enum Suit: string
{
    case Hearts = 'H';
    case Spades = 'S';
}

interface Named {}
trait Greets {}

try {
    $u = new User(1);
    $anon = new class extends User {};
} catch (RuntimeException | LogicException $e) {
    echo $e;
} finally {
    unset($u);
}

class Temperature
{
    public string $fullName {
        get => $this->first . " " . $this->last;
        set(string $value) {
            $this->first = $value;
        }
    }
}

use App\Sub\{First, Second};

$fn = fn($x) => $x * 2;
$result = to_string($now) + strlen($name) - MAX_ITEMS;
$flag = true && false || null;
$obj->prop = $arr["k"];
$magic = __MAGIC__;
echo PHP_EOL, E_ALL;
