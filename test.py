import requests
import json
import time

BASE_URL = "http://localhost:8001"

def test_endpoint(endpoint, code, language, input=""):
    url = f"{BASE_URL}/{endpoint}"
    data = {
        "language": language,
        "code": code,
        "input": input
    }
    
    print(f"\nTesting {endpoint} with {language}:")
    print(f"Code: {code}")
    print(f"Input: {input}")
    
    try:
        response = requests.post(url, json=data)
        print(f"Status Code: {response.status_code}")
        if response.status_code == 200:
            print(f"Response: {json.dumps(response.json(), indent=2)}")
        else:
            print(f"Error Response: {response.text}")
        return response.status_code == 200
    except Exception as e:
        print(f"Error: {str(e)}")
        return False

def main():
    # Test Python with input
    python_code = """
# Read input
n = int(input())
numbers = list(map(int, input().split()))
print(f"Received {n} numbers: {numbers}")
"""
    test_endpoint("execute", python_code, "python", "3\n1 2 3")
    
    # Test C++ with input
    cpp_code = """
#include <iostream>
#include <vector>
using namespace std;

int main() {
    int n;
    cin >> n;
    vector<int> numbers(n);
    for(int i = 0; i < n; i++) {
        cin >> numbers[i];
    }
    cout << "Received " << n << " numbers: ";
    for(int num : numbers) {
        cout << num << " ";
    }
    cout << endl;
    return 0;
}
"""
    test_endpoint("execute", cpp_code, "cpp", "3\n1 2 3")
    
    # Test Java with input
    java_code = """
import java.util.*;
import java.io.*;

public class Main {
    public static void main(String[] args) {
        Scanner scanner = new Scanner(System.in);
        int n = scanner.nextInt();
        int[] numbers = new int[n];
        for(int i = 0; i < n; i++) {
            numbers[i] = scanner.nextInt();
        }
        System.out.print("Received " + n + " numbers: ");
        for(int num : numbers) {
            System.out.print(num + " ");
        }
        System.out.println();
    }
}
"""
    test_endpoint("execute", java_code, "java", "3\n1 2 3")
    
    # Test Go with input
    go_code = """
package main

import (
    "fmt"
    "bufio"
    "os"
    "strings"
    "strconv"
)

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    n, _ := strconv.Atoi(scanner.Text())
    
    scanner.Scan()
    numbersStr := strings.Fields(scanner.Text())
    numbers := make([]int, n)
    for i := 0; i < n; i++ {
        numbers[i], _ = strconv.Atoi(numbersStr[i])
    }
    
    fmt.Printf("Received %d numbers: %v\\n", n, numbers)
}
"""
    test_endpoint("execute", go_code, "go", "3\n1 2 3")
    
    # Test JavaScript with input
    js_code = """
const readline = require('readline');
const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
});

let n;
let numbers = [];

rl.on('line', (line) => {
    if (n === undefined) {
        n = parseInt(line);
    } else {
        numbers = line.split(' ').map(Number);
        console.log(`Received ${n} numbers: ${numbers.join(' ')}`);
        rl.close();
    }
});
"""
    test_endpoint("execute", js_code, "javascript", "3\n1 2 3")
    
    # Test submission endpoint with multiple test cases
    python_submission = """
def process_input(input_str):
    n = int(input_str.split('\\n')[0])
    numbers = list(map(int, input_str.split('\\n')[1].split()))
    return f"Received {n} numbers: {numbers}"

if __name__ == "__main__":
    user_input = input()
    result = process_input(user_input)
    print(result)
"""
    
    test_cases = [
        {
            "input": "3\n1 2 3",
            "expected_output": "Received 3 numbers: [1, 2, 3]"
        },
        {
            "input": "5\n1 2 3 4 5",
            "expected_output": "Received 5 numbers: [1, 2, 3, 4, 5]"
        }
    ]
    
    data = {
        "language": "python",
        "code": python_submission,
        "test_cases": test_cases
    }
    
    test_endpoint("submit", python_submission, "python", json.dumps(test_cases))

if __name__ == "__main__":
    main()
