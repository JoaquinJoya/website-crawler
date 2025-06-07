#!/usr/bin/env python3
"""
AI Processor Script for Web Crawler
Handles AI API calls for OpenAI, Claude, and Gemini
Communication via stdin/stdout for fast Go-Python integration
"""

import json
import sys
import os
from typing import Dict, Any

def process_openai(payload: Dict[str, Any]) -> str:
    """Process content using OpenAI API"""
    try:
        import openai
        
        client = openai.OpenAI(api_key=payload["api_key"])
        model = payload["model"] or "gpt-4o-mini"
        
        response = client.chat.completions.create(
            model=model,
            messages=[
                {
                    "role": "user",
                    "content": f"{payload['prompt']}\n\nContent to analyze:\n{payload['content']}"
                }
            ],
            max_tokens=1000,
            temperature=0.7
        )
        
        return response.choices[0].message.content.strip()
        
    except ImportError:
        return "Error: OpenAI library not installed. Run: pip install openai"
    except Exception as e:
        return f"OpenAI Error: {str(e)}"

def process_claude(payload: Dict[str, Any]) -> str:
    """Process content using Anthropic Claude API"""
    try:
        import anthropic
        
        client = anthropic.Anthropic(api_key=payload["api_key"])
        model = payload["model"] or "claude-3-5-haiku-20241022"
        
        message = client.messages.create(
            model=model,
            max_tokens=1000,
            messages=[
                {
                    "role": "user",
                    "content": f"{payload['prompt']}\n\nContent to analyze:\n{payload['content']}"
                }
            ]
        )
        
        return message.content[0].text.strip()
        
    except ImportError:
        return "Error: Anthropic library not installed. Run: pip install anthropic"
    except Exception as e:
        return f"Claude Error: {str(e)}"

def process_gemini(payload: Dict[str, Any]) -> str:
    """Process content using Google Gemini API"""
    try:
        import google.generativeai as genai
        
        genai.configure(api_key=payload["api_key"])
        model_name = payload["model"] or "gemini-1.5-flash"
        
        model = genai.GenerativeModel(model_name)
        prompt = f"{payload['prompt']}\n\nContent to analyze:\n{payload['content']}"
        
        response = model.generate_content(prompt)
        return response.text.strip()
        
    except ImportError:
        return "Error: Google GenerativeAI library not installed. Run: pip install google-generativeai"
    except Exception as e:
        return f"Gemini Error: {str(e)}"

def main():
    """Main function to process AI requests"""
    try:
        # Read JSON payload from stdin
        input_data = sys.stdin.read().strip()
        if not input_data:
            print("Error: No input data received")
            sys.exit(1)
            
        payload = json.loads(input_data)
        
        # Validate required fields
        required_fields = ["provider", "api_key", "prompt", "content"]
        for field in required_fields:
            if not payload.get(field):
                print(f"Error: Missing required field: {field}")
                sys.exit(1)
        
        provider = payload["provider"].lower()
        
        # Route to appropriate AI provider
        if provider == "openai":
            result = process_openai(payload)
        elif provider == "claude":
            result = process_claude(payload)
        elif provider == "gemini":
            result = process_gemini(payload)
        else:
            result = f"Error: Unknown AI provider: {provider}"
        
        # Output result to stdout
        print(result)
        
    except json.JSONDecodeError as e:
        print(f"Error: Invalid JSON input: {str(e)}")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {str(e)}")
        sys.exit(1)

if __name__ == "__main__":
    main()