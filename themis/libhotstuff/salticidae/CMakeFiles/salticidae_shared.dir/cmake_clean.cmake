file(REMOVE_RECURSE
  "libsalticidae.dylib"
  "libsalticidae.pdb"
)

# Per-language clean rules from dependency scanning.
foreach(lang CXX)
  include(CMakeFiles/salticidae_shared.dir/cmake_clean_${lang}.cmake OPTIONAL)
endforeach()
