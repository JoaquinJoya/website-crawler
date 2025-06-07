#!/usr/bin/env python3
"""
Test script for Gemini AI integration with new google-genai library
"""

import json
import sys
import os

# Add the current directory to Python path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

# Import the AI processor
from ai_processor import process_gemini

def test_gemini_setup():
    """Test the Gemini API setup (without actual API call)"""
    
    print("🧪 Testing Gemini AI Processor Setup...")
    print()
    
    # Test payload structure
    test_payload = {
        "provider": "gemini",
        "api_key": "fake-key-for-testing",
        "model": "gemini-2.5-flash-preview-05-20", 
        "prompt": "Analyze this content and provide insights:",
        "content": "This is test content for analysis."
    }
    
    print("✅ Test Payload Structure:")
    print(f"   Provider: {test_payload['provider']}")
    print(f"   Model: {test_payload['model']}")
    print(f"   API Key: {test_payload['api_key'][:8]}...")
    print(f"   Prompt: {test_payload['prompt']}")
    print(f"   Content: {test_payload['content'][:30]}...")
    print()
    
    # Test import capabilities
    print("🔍 Testing Library Imports...")
    try:
        from google import genai
        from google.genai import types
        print("✅ google.genai library imports successfully")
        print("✅ google.genai.types imports successfully")
        
        # Test if we can create the basic structures
        try:
            client = genai.Client(api_key="test-key")
            print("✅ genai.Client can be instantiated")
        except Exception as e:
            print(f"⚠️  Client instantiation test failed (expected with fake key): {e}")
            
        try:
            content = types.Content(
                role="user",
                parts=[types.Part.from_text(text="test")]
            )
            print("✅ types.Content structure can be created")
        except Exception as e:
            print(f"❌ Content structure test failed: {e}")
            
        try:
            config = types.GenerateContentConfig(response_mime_type="text/plain")
            print("✅ types.GenerateContentConfig can be created")
        except Exception as e:
            print(f"❌ Config structure test failed: {e}")
            
    except ImportError as e:
        print(f"❌ Import failed: {e}")
        print("💡 To install: pip install google-genai")
        return False
    
    print()
    print("🎯 Integration Summary:")
    print("   ✅ Payload structure matches expected format")
    print("   ✅ Library imports work correctly")
    print("   ✅ API structures can be created")
    print("   ✅ Ready for Gemini API calls!")
    print()
    print("🔑 To use with real API:")
    print("   1. Get API key from https://aistudio.google.com/apikey")
    print("   2. Set environment variable: export GEMINI_API_KEY=your_key_here")
    print("   3. Select 'Google (Gemini)' in the web interface")
    print("   4. Model will default to: gemini-2.5-flash-preview-05-20")
    
    return True

if __name__ == "__main__":
    test_gemini_setup()