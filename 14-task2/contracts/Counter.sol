// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Counter {
    uint256 private value;

    event Incremented(address indexed by, uint256 newValue);

    constructor(uint256 init) {
        value = init;
    }

    function current() external view returns (uint256) {
        return value;
    }

    function increment() external {
        value += 1;
        emit Incremented(msg.sender, value);
    }
}
