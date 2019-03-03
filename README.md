# Wikiator

### Automated code wiki generator using comment tags

> Creates/updates a searchable code wiki for reusing code

> Supports parsing & syntax highlighting for all major languages

> Can create software documentations

Demo wiki @ https://logwolvy.github.io/

---

### Usage
1. Say, you have a resuable code snippet. Share it on code wiki by commenting that part of code as shown below:

```ruby
# wiki/ruby/some-descriptive-name
def require_all_files(path)
  $LOAD_PATH.push path # resource path
  rbfiles = Dir.entries(path).select { |x| /\.rb\z/ =~ x }
  rbfiles -= [File.basename(__FILE__)]
  rbfiles.each do |path|
    require(File.basename(path))
  end
end
# end-wiki
```

2. Download the `wikiator` binary. Though optional, would be good if you put it in your loadpath

3. If Git watch mode not setup, just run `wikiator -manual-mode AbsPath/to/your/repo`. This will update & deploy the code wiki

---

#### Git watch mode setup (Optional)
```bash
wikiator -setup=AbsPath/to/your/repo
# This will append/create a pre-commit hook in your repo
```
