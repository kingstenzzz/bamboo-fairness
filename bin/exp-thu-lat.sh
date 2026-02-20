#!/bin/bash


node=(8 )
algorithms=( "Default" "HyperG" )

for n in "${node[@]}"; do
  # 内层循环
  for algorithm in "${algorithms[@]}"; do
    echo "Running exp.sh with node=$n and algorithm=$algorithm"
    ./makeIp_local.sh $n
    #./solo-exp.sh "$algorithm"
    ./solo-banchmark.sh "$algorithm"
    sleep 10
  done
done
