Name:       dedup
Version:    1
Release:    1
Summary:    Deduplicate files and store them in a canonical location.
License:    MIT

%description
dedup deduplicates files by storing a unique copy in a canonical directory and replacing them with softlinks.

%prep
# we have no source, so nothing here

%build
go build .

%install
mkdir -p %{buildroot}/usr/bin/
install -m 755 go-dedup %{buildroot}/usr/bin/dedup

%files
/usr/bin/dedup

%changelog
# let's skip this for now
