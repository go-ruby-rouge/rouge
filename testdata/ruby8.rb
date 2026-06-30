module M
  class C < Base
    def self.make(x)
      x.map { |i| i * 2 }
    end
  end
end
result = ok ? :yes : :no
val = 1.divmod 2
