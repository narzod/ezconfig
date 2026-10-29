Windows resources for ezconfig.exe: version details (shown under
Properties > Details) and a manifest. Unsigned programs without these are
more likely to be flagged by antivirus heuristics.

After changing the version or the manifest, rebuild the object that Go
links into Windows builds, from the folder above this one:

    x86_64-w64-mingw32-windres --preprocessor=cat -O coff -o rsrc_windows_amd64.syso winres/ezconfig.rc

On Windows with MinGW installed, use windres instead of the prefixed name.
