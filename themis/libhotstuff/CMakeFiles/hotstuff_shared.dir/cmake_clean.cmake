file(REMOVE_RECURSE
  "libhotstuff.dylib"
  "libhotstuff.pdb"
)

# Per-language clean rules from dependency scanning.
foreach(lang CXX)
  include(CMakeFiles/hotstuff_shared.dir/cmake_clean_${lang}.cmake OPTIONAL)
endforeach()
